"""Renders values read by fastavro in the one-line form that std/avro's diagnostic prints."""
import datetime
import decimal
import json
import struct
import uuid


def hexs(b):
    return b.hex()


def jstr(s):
    return json.dumps(s, ensure_ascii=False)


class Names:
    def __init__(self):
        self.types = {}

    def register(self, schema, namespace=""):
        if isinstance(schema, list):
            for b in schema:
                self.register(b, namespace)
        elif isinstance(schema, dict):
            t = schema["type"]
            if t in ("record", "enum", "fixed"):
                name = schema["name"]
                ns = schema.get("namespace", namespace) if "." not in name else name.rsplit(".", 1)[0]
                full = name if "." in name else (f"{ns}.{name}" if ns else name)
                self.types[full] = schema
                self.types[name.rsplit(".", 1)[-1]] = schema
                if t == "record":
                    for f in schema["fields"]:
                        self.register(f["type"], ns)
            elif t == "array":
                self.register(schema["items"], namespace)
            elif t == "map":
                self.register(schema["values"], namespace)
            elif isinstance(t, (dict, list)):
                self.register(t, namespace)

    def resolve(self, schema):
        if isinstance(schema, str) and schema in self.types:
            return self.types[schema]
        return schema


EPOCH_DATE = datetime.date(1970, 1, 1)
EPOCH_UTC = datetime.datetime(1970, 1, 1, tzinfo=datetime.timezone.utc)
EPOCH_NAIVE = datetime.datetime(1970, 1, 1)


def dump_float(x):
    return "f" + struct.pack(">d", x).hex()


def py_matches(v, branch, names):
    b = names.resolve(branch)
    t = b["type"] if isinstance(b, dict) else b
    if isinstance(b, dict) and b.get("logicalType"):
        return v is not None
    if t == "null":
        return v is None
    if t == "boolean":
        return isinstance(v, bool)
    if t in ("int", "long"):
        return isinstance(v, int) and not isinstance(v, bool)
    if t in ("float", "double"):
        return isinstance(v, float)
    if t in ("bytes", "fixed"):
        return isinstance(v, (bytes, bytearray))
    if t in ("string", "enum"):
        return isinstance(v, str)
    if t == "array":
        return isinstance(v, list)
    if t in ("map", "record"):
        return isinstance(v, dict)
    return False


def dump(v, schema, names):
    schema = names.resolve(schema)
    if isinstance(schema, list):
        if v is None:
            return "null"
        branches = [b for b in schema if b != "null"]
        for b in branches:
            if py_matches(v, b, names):
                return dump(v, b, names)
        raise ValueError(f"no branch for {v!r}")
    if isinstance(schema, dict):
        t = schema["type"]
        logical = schema.get("logicalType")
        if isinstance(t, (dict, list)):
            return dump(v, t, names)
    else:
        t = schema
        logical = None
    if t == "null":
        return "null"
    if t == "boolean":
        return "true" if v else "false"
    if t in ("int", "long"):
        if logical == "date":
            return f"date:{(v - EPOCH_DATE).days}"
        if logical == "time-millis":
            return "time-millis:%d" % (((v.hour * 60 + v.minute) * 60 + v.second) * 1000 + v.microsecond // 1000)
        if logical == "time-micros":
            return "time-micros:%d" % (((v.hour * 60 + v.minute) * 60 + v.second) * 1000000 + v.microsecond)
        if logical == "timestamp-millis":
            return "timestamp-millis:%d" % ((v - EPOCH_UTC) // datetime.timedelta(milliseconds=1))
        if logical == "timestamp-micros":
            return "timestamp-micros:%d" % ((v - EPOCH_UTC) // datetime.timedelta(microseconds=1))
        if logical == "timestamp-nanos":
            return f"timestamp-nanos:{v}"
        if logical == "local-timestamp-millis":
            return "local-timestamp-millis:%d" % ((v - EPOCH_NAIVE) // datetime.timedelta(milliseconds=1))
        if logical == "local-timestamp-micros":
            return "local-timestamp-micros:%d" % ((v - EPOCH_NAIVE) // datetime.timedelta(microseconds=1))
        if logical == "local-timestamp-nanos":
            return f"local-timestamp-nanos:{v}"
        return str(v)
    if t in ("float", "double"):
        return dump_float(v)
    if t == "bytes":
        if logical == "decimal":
            scale = schema.get("scale", 0)
            return f"decimal({format(v.quantize(decimal.Decimal(1).scaleb(-scale)), 'f')})"
        return f'x"{hexs(v)}"'
    if t == "fixed":
        if logical == "duration":
            return "duration:%d,%d,%d" % struct.unpack("<III", v)
        if logical == "uuid":
            return f"uuid:{uuid.UUID(bytes=v)}"
        if logical == "decimal":
            scale = schema.get("scale", 0)
            if isinstance(v, bytes):
                n = int.from_bytes(v, "big", signed=True)
                v = decimal.Decimal(n).scaleb(-scale)
            return f"decimal({format(v.quantize(decimal.Decimal(1).scaleb(-scale)), 'f')})"
        return f'x"{hexs(v)}"'
    if t == "string":
        if logical == "uuid":
            return f"uuid:{v}"
        return jstr(v)
    if t == "enum":
        return jstr(v)
    if t == "array":
        return "[" + ", ".join(dump(x, schema["items"], names) for x in v) + "]"
    if t == "map":
        return "{" + ", ".join(f"{jstr(k)}: {dump(x, schema['values'], names)}" for k, x in v.items()) + "}"
    if t == "record":
        return "{" + ", ".join(f"{jstr(f['name'])}: {dump(v[f['name']], f['type'], names)}" for f in schema["fields"]) + "}"
    raise ValueError(f"cannot dump {t}")


def named(schema):
    n = Names()
    n.register(schema)
    return n
