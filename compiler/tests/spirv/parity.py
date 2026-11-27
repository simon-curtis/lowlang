"""Runs the kernels of testdata/run/kernel.low in interp.py as its main does.

It compares each value with the line the native build prints for it in
testdata/run/kernel.out, then checks the three failures a module reports
in its flag buffer.

usage: parity.py <directory of .spv files> <testdata/run/kernel.out>
"""
import os
import struct
import sys

sys.dont_write_bytecode = True

import interp


def floats(values):
    return [struct.unpack("<I", struct.pack("<f", x))[0] for x in values]


def longs(values):
    out = []
    for x in values:
        out += [x & 0xFFFFFFFF, x >> 32 & 0xFFFFFFFF]
    return out


def single(x):
    return floats([x])[0]


def double(x):
    return struct.unpack("<Q", struct.pack("<d", x))[0]


def as_floats(cells):
    return [struct.unpack("<f", struct.pack("<I", c))[0] for c in cells]


def as_longs(cells):
    out = []
    for i in range(0, len(cells), 2):
        x = cells[i] | cells[i + 1] << 32
        out.append(x - (1 << 64) if x >> 63 else x)
    return out


class Run:
    def __init__(self, directory, name, scalars, slices):
        self.module = interp.load(os.path.join(directory, name + ".spv"))
        self.name = name
        self.scalars = scalars
        self.buffers = {0: [0]}
        self.lengths = []
        for i, (cells, length) in enumerate(slices):
            self.buffers[i + 1] = cells
            self.lengths.append(length)

    def invoke(self, *ids):
        for invocation in ids:
            self.module.run(invocation, [1 << 20] + self.scalars + self.lengths, self.buffers)
        return self

    def flag(self):
        return self.buffers[0][0]


class Checks:
    def __init__(self, printed):
        with open(printed) as f:
            self.lines = f.read().split()
        self.at = 0
        self.failed = 0

    def printed(self, run, got):
        want = self.lines[self.at]
        self.at += 1
        if isinstance(got, float):
            want = interp.f32(float(want))
        else:
            want = int(want)
        self.same("%s, line %d" % (run.name, self.at), got, want)
        self.same("%s, flag" % run.name, run.flag(), 0)

    def same(self, what, got, want):
        if got != want:
            self.failed += 1
            print("FAIL %s: the module gives %s, want %s" % (what, got, want))
            return


def main(d, printed):
    c = Checks(printed)
    xs = floats([1.0, 2.0, 3.0, 4.0])
    ys = floats([10.0, 20.0, 30.0, 40.0])
    saxpy = Run(d, "saxpy", [single(2.0)], [(list(xs), 4), (ys, 4)]).invoke(0, 1, 2, 3)
    c.printed(saxpy, as_floats(ys)[3])
    ps = floats([2.0]) + [0] + floats([4.0]) + [0, 0, 0]
    step = Run(d, "step", [1, single(3.0)], [(ps, 1)]).invoke(0)
    c.printed(step, as_floats(ps)[0])
    c.printed(step, ps[4])
    out = floats([0.0, 0.0, 0.0, 0.0])
    blur = Run(d, "blur", [2, 2], [(list(xs), 4), (out, 4)]).invoke(0, 1, 2, 3)
    c.printed(blur, as_floats(out)[0])
    edges = Run(d, "edges", [], [(out, 4)]).invoke(3)
    c.printed(edges, as_floats(out)[0])
    c.printed(edges, as_floats(out)[3])
    lanes = Run(d, "lanes", [], [(list(xs), 4), (ys, 4)]).invoke(0)
    c.printed(lanes, as_floats(ys)[3])
    zs = floats([1.5])
    grow = Run(d, "grow", [], [(zs, 1)]).invoke(0)
    c.printed(grow, as_floats(zs)[0])
    found = longs([0, 0])
    search = Run(d, "search", [3], [([0, 3, 4], 3), (found, 2)]).invoke(0, 1)
    c.printed(search, as_longs(found)[0])
    c.printed(search, as_longs(found)[1])
    hits = [0, 0, 0, 0]
    grid = Run(d, "grid", [2], [([0, 7, 1, 200], 4), (hits, 4)]).invoke(1)
    c.printed(grid, hits[1])
    sums = floats([0.0, 0.0])
    choose = Run(d, "choose", [], [(floats([1.0, 2.0]), 2), (floats([5.0]), 1), (sums, 2)]).invoke(0, 1)
    c.printed(choose, as_floats(sums)[0])
    c.printed(choose, as_floats(sums)[1])
    cells = [0, 0, 0, 0]
    wide = Run(d, "wide", [-5 & (2**64 - 1), double(0.5)], [(longs([40]), 1), (cells, 1)]).invoke(0)
    c.printed(wide, as_longs(cells[0:2])[0])
    c.printed(wide, cells[2])
    ends = longs([0, 0, 0, 0])
    limits = Run(d, "limits", [64, double(1.0e30)], [(longs([-1099511627776]), 1), (ends, 4)]).invoke(0)
    for end in as_longs(ends):
        c.printed(limits, end)
    steps = floats([0.0])
    early = Run(d, "early", [single(10.0)], [(floats([3.0]), 1), (steps, 1)]).invoke(0)
    c.printed(early, as_floats(steps)[0])
    bits = [0]
    mask = Run(d, "mask", [], [([1, 2, 3, 4], 4), (bits, 1)]).invoke(0)
    c.printed(mask, bits[0])
    c.same("every printed line is checked", c.at, len(c.lines))

    past = Run(d, "saxpy", [single(2.0)], [(list(xs), 4), (floats([1.0]), 1)]).invoke(2)
    c.same("saxpy past the end of y, flag", past.flag(), 1)
    zero = Run(d, "wide", [2, double(0.5)], [(longs([24]), 1), ([0, 0, 0, 0], 1)]).invoke(0)
    c.same("wide dividing by zero, flag", zero.flag(), 2)
    beyond = Run(d, "saxpy", [single(2.0)], [(list(xs), 4), (floats([7.0]), 1)])
    beyond.module.run(5, [3, single(2.0), 4, 1], beyond.buffers)
    c.same("saxpy at an index past the count, flag", beyond.flag(), 0)
    c.same("saxpy at an index past the count, y", as_floats(beyond.buffers[2]), [7.0])
    return c.failed


if __name__ == "__main__":
    sys.exit(1 if main(sys.argv[1], sys.argv[2]) else 0)
