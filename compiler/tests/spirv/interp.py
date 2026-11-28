"""Runs a compute module from compiler/spirv one invocation at a time.

It implements the instructions that back end writes, from the SPIR-V
specification, and nothing of the back end's own logic.
"""
import math
import struct
import sys


def f32(x):
    try:
        return struct.unpack("<f", struct.pack("<f", x))[0]
    except OverflowError:
        return math.copysign(math.inf, x)


def bits_of(value, kind):
    if kind[0] == "float":
        return struct.unpack("<I" if kind[1] == 32 else "<Q", struct.pack("<f" if kind[1] == 32 else "<d", value))[0]
    return int(value)


def from_bits(raw, kind):
    if kind[0] == "float":
        return struct.unpack("<f" if kind[1] == 32 else "<d", struct.pack("<I" if kind[1] == 32 else "<Q", raw))[0]
    return raw


def signed(x, width):
    return x - (1 << width) if x >> (width - 1) else x


class Returned(Exception):
    pass


class Module:
    def __init__(self, data):
        words = struct.unpack("<%dI" % (len(data) // 4), data)
        assert words[0] == 0x07230203
        self.types = {}
        self.constants = {}
        self.globals = {}
        self.bindings = {}
        self.builtins = {}
        self.code = []
        at = 5
        in_function = False
        while at < len(words):
            count, op = words[at] >> 16, words[at] & 0xFFFF
            ops = words[at + 1:at + count]
            at += count
            if op == 54:
                in_function = True
                continue
            if in_function:
                self.code.append((op, ops))
                continue
            self.declare(op, ops)

    def declare(self, op, ops):
        t = self.types
        if op == 19:
            t[ops[0]] = ("void",)
        elif op == 20:
            t[ops[0]] = ("bool",)
        elif op == 21:
            t[ops[0]] = ("int", ops[1])
        elif op == 22:
            t[ops[0]] = ("float", ops[1])
        elif op == 23:
            t[ops[0]] = ("vector", ops[1], ops[2])
        elif op == 28:
            t[ops[0]] = ("array", ops[1], self.constants[ops[2]])
        elif op == 29:
            t[ops[0]] = ("runtime", ops[1])
        elif op == 30:
            t[ops[0]] = ("struct", ops[1:])
        elif op == 32:
            t[ops[0]] = ("pointer", ops[1], ops[2])
        elif op == 33:
            t[ops[0]] = ("function",)
        elif op == 43:
            kind = t[ops[0]]
            raw = ops[2] | (ops[3] << 32 if len(ops) > 3 else 0)
            self.constants[ops[1]] = from_bits(raw, kind)
        elif op == 46:
            self.constants[ops[1]] = self.zero(ops[0])
        elif op == 59:
            self.globals[ops[1]] = (t[ops[0]][2], ops[2])
        elif op == 71:
            if ops[1] == 33:
                self.bindings[ops[0]] = ops[2]
            if ops[1] == 11:
                self.builtins[ops[0]] = ops[2]
        elif op in (11, 14, 15, 16, 17, 72):
            pass
        else:
            raise SystemExit("declaration %d is not implemented" % op)

    def zero(self, type_id):
        kind = self.types[type_id]
        if kind[0] == "array":
            return [self.zero(kind[1]) for _ in range(kind[2])]
        if kind[0] == "float":
            return 0.0
        return 0

    def run(self, invocation, constants, buffers):
        """buffers maps a binding to a list of 32-bit cells; constants lists the push constant members."""
        values = dict(self.constants)
        for variable, (_, storage) in self.globals.items():
            if storage == 1:
                values[variable] = ([[invocation, 0, 0]], 0)
            elif storage == 9:
                values[variable] = ([list(constants)], 0)
            else:
                values[variable] = ([[buffers[self.bindings[variable]]]], 0)
        labels = {ops[0]: i for i, (op, ops) in enumerate(self.code) if op == 248}
        at = 0
        current = previous = None
        steps = 0
        try:
            while True:
                steps += 1
                if steps > 5_000_000:
                    raise SystemExit("the invocation does not end")
                op, ops = self.code[at]
                at += 1
                if op == 248:
                    previous, current = current, ops[0]
                elif op == 249:
                    at = labels[ops[0]]
                elif op == 250:
                    at = labels[ops[1] if values[ops[0]] else ops[2]]
                elif op == 251:
                    target = ops[1]
                    for i in range(2, len(ops), 2):
                        if ops[i] == values[ops[0]]:
                            target = ops[i + 1]
                    at = labels[target]
                elif op == 253:
                    raise Returned()
                elif op == 255:
                    raise SystemExit("reached OpUnreachable")
                elif op in (0, 246, 247, 56):
                    pass
                elif op == 245:
                    for i in range(2, len(ops), 2):
                        if ops[i + 1] == previous:
                            values[ops[1]] = values[ops[i]]
                else:
                    self.step(op, ops, values)
        except Returned:
            return

    def step(self, op, ops, v):
        t = self.types
        if op == 59:
            initial = v[ops[3]] if len(ops) > 3 else None
            if isinstance(initial, list):
                initial = list(initial)
            v[ops[1]] = ([initial], 0)
            return
        if op == 62:
            container, key = v[ops[0]]
            container[key] = v[ops[1]]
            return
        kind = t[ops[0]]
        out = ops[1]
        a = [v[x] if x in v else x for x in ops[2:]]
        width = kind[1] if kind[0] in ("int", "float") else 0
        mask = (1 << width) - 1 if kind[0] == "int" else 0

        def operand_width(i):
            return self.width_of(ops[2 + i], v)

        if op == 61:
            container, key = a[0]
            value = container[key]
            if value is None:
                raise SystemExit("load of a variable that was never stored")
            v[out] = value
        elif op == 65:
            container, key = a[0]
            for index in a[1:]:
                container, key = container[key], index
                if key >= len(container):
                    raise SystemExit("access past the end: index %d of %d" % (key, len(container)))
            v[out] = (container, key)
        elif op == 81:
            v[out] = a[0][ops[3]]
        elif op == 12:
            v[out] = self.extended(ops[3], v[ops[4]], kind, self.width_of(ops[4], v))
        elif op == 113:
            v[out] = a[0] & mask
        elif op == 114:
            v[out] = signed(a[0], operand_width(0)) & mask
        elif op == 110:
            if math.isnan(a[0]) or not -(1 << (width - 1)) <= a[0] < (1 << (width - 1)):
                raise SystemExit("conversion of %s to a %d-bit integer" % (a[0], width))
            v[out] = int(a[0]) & mask
        elif op == 111:
            x = float(signed(a[0], operand_width(0)))
            v[out] = f32(x) if width == 32 else x
        elif op == 112:
            v[out] = f32(float(a[0])) if width == 32 else float(a[0])
        elif op == 115:
            v[out] = f32(a[0]) if width == 32 else a[0]
        elif op == 124:
            v[out] = from_bits(bits_of(a[0], self.kind_of(ops[2], v)), kind)
        elif op == 127:
            v[out] = -a[0]
        elif op in (128, 130, 132):
            v[out] = {128: a[0] + a[1], 130: a[0] - a[1], 132: a[0] * a[1]}[op] & mask
        elif op in (129, 131, 133, 136):
            if op == 136:
                if a[1] == 0:
                    x = math.nan if a[0] == 0 or math.isnan(a[0]) else math.copysign(math.inf, a[0]) * math.copysign(1, a[1])
                else:
                    x = a[0] / a[1]
            else:
                x = {129: a[0] + a[1], 131: a[0] - a[1], 133: a[0] * a[1]}[op]
            v[out] = f32(x) if width == 32 else x
        elif op in (134, 137):
            if a[1] == 0:
                raise SystemExit("division by zero")
            v[out] = a[0] // a[1] if op == 134 else a[0] % a[1]
        elif op in (135, 138):
            x, y = signed(a[0], width), signed(a[1], width)
            if y == 0 or (y == -1 and x == -(1 << (width - 1))):
                raise SystemExit("signed division of %d by %d" % (x, y))
            q = abs(x) // abs(y) * (1 if (x < 0) == (y < 0) else -1)
            v[out] = (q if op == 135 else x - q * y) & mask
        elif op == 151:
            w = self.types[kind[1][0]][1]
            v[out] = [(a[0] * a[1]) & ((1 << w) - 1), (a[0] * a[1]) >> w]
        elif op == 156:
            v[out] = math.isnan(a[0])
        elif op == 166:
            v[out] = a[0] or a[1]
        elif op == 167:
            v[out] = a[0] and a[1]
        elif op == 169:
            v[out] = a[1] if a[0] else a[2]
        elif op in (170, 171):
            v[out] = (a[0] == a[1]) == (op == 170)
        elif op in (172, 174, 176, 178):
            v[out] = {172: a[0] > a[1], 174: a[0] >= a[1], 176: a[0] < a[1], 178: a[0] <= a[1]}[op]
        elif op in (173, 175, 177, 179):
            w = operand_width(0)
            x, y = signed(a[0], w), signed(a[1], w)
            v[out] = {173: x > y, 175: x >= y, 177: x < y, 179: x <= y}[op]
        elif op in (180, 184, 186, 188, 190):
            v[out] = {180: a[0] == a[1], 184: a[0] < a[1], 186: a[0] > a[1], 188: a[0] <= a[1], 190: a[0] >= a[1]}[op]
        elif op == 183:
            v[out] = a[0] != a[1]
        elif op in (194, 195, 196):
            if a[1] >= width:
                raise SystemExit("shift by %d of a %d-bit value" % (a[1], width))
            if op == 194:
                v[out] = a[0] >> a[1]
            elif op == 195:
                v[out] = (signed(a[0], width) >> a[1]) & mask
            else:
                v[out] = (a[0] << a[1]) & mask
        elif op in (197, 198, 199):
            v[out] = {197: a[0] | a[1], 198: a[0] ^ a[1], 199: a[0] & a[1]}[op]
        else:
            raise SystemExit("instruction %d is not implemented" % op)

    def kind_of(self, value_id, v):
        return self.value_types[value_id]

    def width_of(self, value_id, v):
        return self.value_types[value_id][1]

    def extended(self, instruction, x, kind, operand_width):
        if instruction == 2:
            r = float(round(x))
            return r
        if instruction == 8:
            return float(math.floor(x)) if math.isfinite(x) else x
        if instruction == 31:
            r = math.sqrt(x) if x >= 0 else math.nan
            return f32(r) if kind[1] == 32 else r
        if instruction == 73:
            if x == 0:
                return (1 << operand_width) - 1
            return (x & -x).bit_length() - 1
        raise SystemExit("extended instruction %d is not implemented" % instruction)

    def index_types(self):
        self.value_types = {}
        for constant in self.constants:
            pass
        for op, ops in self.code:
            if op in (248, 249, 250, 251, 253, 255, 0, 246, 247, 56, 62):
                continue
            if ops[0] in self.types:
                self.value_types[ops[1]] = self.types[ops[0]]


def load(path):
    with open(path, "rb") as f:
        data = f.read()
    module = Module(data)
    module.index_types()
    words = struct.unpack("<%dI" % (len(data) // 4), data)
    at = 5
    while at < len(words):
        count, op = words[at] >> 16, words[at] & 0xFFFF
        if op in (43, 46):
            module.value_types[words[at + 2]] = module.types[words[at + 1]]
        at += count
    return module


if __name__ == "__main__":
    load(sys.argv[1])
    print("loaded")
