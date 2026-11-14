"""Formats pyarrow tables in the lines that main.low prints for std/arrow.

Everything here reads values through pyarrow's public scalar and type API, never through Low.
"""
import gzip
import hashlib
import json
import struct

import pyarrow as pa


def jstr(s):
    return json.dumps(s, ensure_ascii=False)


def float_text(x):
    return "f" + struct.pack(">d", float(x)).hex()


def is_interval(t):
    return str(t) in ("month_interval", "day_time_interval", "month_day_nano_interval")


def type_name(t):
    ty = pa.types
    if ty.is_null(t):
        return "null"
    if ty.is_boolean(t):
        return "bool"
    if ty.is_integer(t):
        return f"{'int' if ty.is_signed_integer(t) else 'uint'}{t.bit_width}"
    if ty.is_floating(t):
        return f"float{t.bit_width}"
    if ty.is_string(t):
        return "utf8"
    if ty.is_large_string(t):
        return "large_utf8"
    if ty.is_binary(t):
        return "binary"
    if ty.is_large_binary(t):
        return "large_binary"
    if ty.is_fixed_size_binary(t) and not ty.is_decimal(t):
        return f"fixed_size_binary[{t.byte_width}]"
    if ty.is_decimal(t):
        return f"decimal128({t.precision}, {t.scale})"
    if ty.is_date32(t):
        return "date32"
    if ty.is_date64(t):
        return "date64"
    if ty.is_time(t):
        return f"time{t.bit_width}[{t.unit}]"
    if ty.is_timestamp(t):
        if t.tz:
            return f"timestamp[{t.unit}, tz={t.tz}]"
        return f"timestamp[{t.unit}]"
    if ty.is_duration(t):
        return f"duration[{t.unit}]"
    if is_interval(t):
        return str(t)
    if ty.is_large_list(t):
        return f"large_list<{type_name(t.value_type)}>"
    if ty.is_list(t):
        return f"list<{type_name(t.value_type)}>"
    if ty.is_fixed_size_list(t):
        return f"fixed_size_list<{type_name(t.value_type)}>[{t.list_size}]"
    if ty.is_struct(t):
        return "struct"
    if ty.is_map(t):
        return "map"
    if ty.is_union(t):
        return "sparse_union" if t.mode == "sparse" else "dense_union"
    if ty.is_dictionary(t):
        return f"dictionary<{type_name(t.value_type)}, {type_name(t.index_type)}>"
    raise SystemExit(f"no name for {t}")


def children(t):
    ty = pa.types
    if ty.is_dictionary(t):
        return children(t.value_type)
    if ty.is_map(t):
        return [pa.field("entries", pa.struct([t.key_field, t.item_field]), nullable=False)]
    return [t.field(i) for i in range(t.num_fields)]


def meta_text(m):
    if not m:
        return ""
    pairs = sorted((k.decode(), v.decode()) for k, v in m.items())
    return " meta " + ",".join(f"{k}={v}" for k, v in pairs)


def type_detail(t):
    ty = pa.types
    if ty.is_union(t):
        return " ids=" + ",".join(str(c) for c in t.type_codes)
    if ty.is_map(t):
        return " keys_sorted" if t.keys_sorted else ""
    if ty.is_dictionary(t):
        return " ordered" if t.ordered else ""
    return ""


def field_lines(f, indent):
    pad = "  " * indent
    out = [f"{pad}field {f.name}: {type_name(f.type)}{type_detail(f.type)} {'nullable' if f.nullable else 'required'}{meta_text(f.metadata)}"]
    for c in children(f.type):
        out.extend(field_lines(c, indent + 1))
    return out


def schema_lines(schema):
    out = [f"schema{meta_text(schema.metadata)}"]
    for f in schema:
        out.extend(field_lines(f, 1))
    return out


def fmt(s):
    t = s.type
    ty = pa.types
    if not s.is_valid:
        return "null"
    if ty.is_null(t):
        return "null"
    if ty.is_boolean(t):
        return "true" if s.as_py() else "false"
    if ty.is_integer(t):
        return str(s.as_py())
    if ty.is_floating(t):
        return float_text(s.as_py())
    if ty.is_string(t) or ty.is_large_string(t):
        return jstr(s.as_py())
    if ty.is_binary(t) or ty.is_large_binary(t) or (ty.is_fixed_size_binary(t) and not ty.is_decimal(t)):
        return 'x"' + s.as_py().hex() + '"'
    if ty.is_decimal(t):
        return f"decimal({format(s.as_py(), 'f')})"
    if ty.is_date32(t):
        return f"date32:{s.cast(pa.int32()).as_py()}"
    if ty.is_date64(t):
        return f"date64:{s.cast(pa.int64()).as_py()}"
    if ty.is_time(t):
        return f"time{t.bit_width}:{s.cast(pa.int32() if t.bit_width == 32 else pa.int64()).as_py()}"
    if ty.is_timestamp(t):
        return f"timestamp:{s.cast(pa.int64()).as_py()}"
    if ty.is_duration(t):
        return f"duration:{s.cast(pa.int64()).as_py()}"
    if is_interval(t):
        v = s.as_py()
        return f"interval:{v.months},{v.days},{v.nanoseconds}"
    raise SystemExit(f"no scalar format for {t}")


def fmt_values(a):
    t = a.type
    ty = pa.types
    n = len(a)
    if n == 0:
        return []
    if ty.is_union(t):
        codes = a.type_codes.to_pylist()
        slots = {code: k for k, code in enumerate(t.type_codes)}
        columns = [fmt_values(a.field(k)) for k in range(t.num_fields)]
        if t.mode == "dense":
            offsets = a.offsets.to_pylist()
            return [f"{codes[i]}={columns[slots[codes[i]]][offsets[i]]}" for i in range(n)]
        return [f"{codes[i]}={columns[slots[codes[i]]][i]}" for i in range(n)]
    valid = a.is_valid().to_pylist()
    if ty.is_list(t) or ty.is_large_list(t):
        offsets = a.offsets.to_pylist()
        values = fmt_values(a.values)
        return ["[" + ", ".join(values[offsets[i] : offsets[i + 1]]) + "]" if valid[i] else "null" for i in range(n)]
    if ty.is_fixed_size_list(t):
        values = fmt_values(a.values)
        size = t.list_size
        return ["[" + ", ".join(values[i * size : (i + 1) * size]) + "]" if valid[i] else "null" for i in range(n)]
    if ty.is_map(t):
        offsets = a.offsets.to_pylist()
        keys = fmt_values(a.keys)
        items = fmt_values(a.items)
        return ["map[" + ", ".join(f"{keys[k]}: {items[k]}" for k in range(offsets[i], offsets[i + 1])) + "]" if valid[i] else "null" for i in range(n)]
    if ty.is_struct(t):
        columns = [fmt_values(a.field(k)) for k in range(t.num_fields)]
        names = [jstr(t.field(k).name) for k in range(t.num_fields)]
        return ["{" + ", ".join(f"{names[k]}: {columns[k][i]}" for k in range(t.num_fields)) + "}" if valid[i] else "null" for i in range(n)]
    if ty.is_dictionary(t):
        indices = a.indices.to_pylist()
        values = fmt_values(a.dictionary)
        return ["null" if indices[i] is None else values[indices[i]] for i in range(n)]
    if str(t) in ("month_interval", "day_time_interval"):
        return interval_values(a)
    return [fmt(a[i]) for i in range(n)]


def interval_values(a):
    name = str(a.type)
    bitmap = a.buffers()[0]
    data = a.buffers()[1].to_pybytes()
    width = {"month_interval": 4, "day_time_interval": 8}[name]
    out = []
    for i in range(len(a)):
        if bitmap is not None and not (bitmap.to_pybytes()[i >> 3] >> (i & 7)) & 1:
            out.append("null")
        elif name == "month_interval":
            out.append(f"interval:{struct.unpack_from('<i', data, i * width)[0]}")
        else:
            days, millis = struct.unpack_from("<ii", data, i * width)
            out.append(f"interval:{days},{millis}")
    return out


def array_line(a):
    return "[" + ", ".join(fmt_values(a)) + "]"


def column_text(line, digest):
    if not digest:
        return line
    return f"sha256 {hashlib.sha256(line.encode()).hexdigest()}"


def batch_lines(batch, digest=False):
    out = [f"batch rows {batch.num_rows}"]
    for i, f in enumerate(batch.schema):
        col = batch.column(i)
        out.append(f"  {f.name} {type_name(f.type)} {column_text(array_line(col), digest)}")
    return out


def table_lines(schema, batches, digest=False):
    out = schema_lines(schema)
    out.append(f"batches {len(batches)}")
    for b in batches:
        out.extend(batch_lines(b, digest))
    return out


def json_value(kind, unit, v):
    if v is None:
        return "null"
    if kind == "duration":
        return f"duration:{v}"
    if unit == "YEAR_MONTH":
        return f"interval:{v}"
    return f"interval:{v['days']},{v['milliseconds']}"


def json_interval_lines(path):
    """Expected lines for the integration file generated_interval, from its JSON golden values (pyarrow cannot wrap its month and day-time interval arrays)."""
    with gzip.open(path, "rt") as f:
        doc = json.load(f)
    names = {"SECOND": "s", "MILLISECOND": "ms", "MICROSECOND": "us", "NANOSECOND": "ns"}
    fields = doc["schema"]["fields"]
    out = ["schema"]
    types = []
    for f in fields:
        t = f["type"]
        if t["name"] == "duration":
            types.append(f"duration[{names[t['unit']]}]")
        else:
            types.append("month_interval" if t["unit"] == "YEAR_MONTH" else "day_time_interval")
        out.append(f"  field {f['name']}: {types[-1]} {'nullable' if f['nullable'] else 'required'}")
    out.append(f"batches {len(doc['batches'])}")
    for b in doc["batches"]:
        out.append(f"batch rows {b['count']}")
        for i, c in enumerate(b["columns"]):
            t = fields[i]["type"]
            values = [json_value(t["name"], t.get("unit"), v if valid else None) for v, valid in zip(c["DATA"], c["VALIDITY"])]
            out.append(f"  {c['name']} {types[i]} [{', '.join(values)}]")
    return out
