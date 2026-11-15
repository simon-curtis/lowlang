"""Generates the binary inputs of std_avro and the expected output ../std_avro.out.

Requires fastavro 1.13.1 and zstandard and python-snappy:

    python3 gen.py

The "writing" section is the digests that verify_written.py recorded for the files std/avro wrote; the fixture fails if they change.

Every expected line comes from fastavro, hashlib and struct, never from the Low code. The CRC-64-AVRO fingerprint is computed from the specification's algorithm (the value for "int" is 7275d51a3f395c8f) because fastavro prints that fingerprint with its bytes reversed.
"""
import datetime
import decimal
import hashlib
import io
import json
import os
import struct
import uuid

import fastavro
from fastavro import parse_schema, reader, schemaless_reader, schemaless_writer, writer
from fastavro.schema import fingerprint, to_parsing_canonical_form

from avrodump import dump, dump_float, hexs, jstr, named

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = []


def emit(line):
    OUT.append(line)


def path(name):
    return os.path.join(HERE, name)


def load(name):
    with open(path(name)) as f:
        return json.load(f)


AVRO_EMPTY = 0xC15D213AA4D7A795


def crc64_table():
    table = []
    for i in range(256):
        fp = i
        for _ in range(8):
            fp = (fp >> 1) ^ (AVRO_EMPTY & -(fp & 1))
        table.append(fp)
    return table


CRC64 = crc64_table()


def rabin(data):
    fp = AVRO_EMPTY
    for x in data:
        fp = (fp >> 8) ^ CRC64[(fp ^ x) & 0xFF]
    return fp


def canonical_lines(label, schema):
    c = to_parsing_canonical_form(schema)
    emit(f"canonical {label} {c}")
    emit(f"fingerprint {label} {rabin(c.encode()):016x} {fingerprint(c, 'md5')} {fingerprint(c, 'sha256')}")
    return c


def section(title):
    emit(f"== {title}")


section("long")
for n in [0, -1, 1, -2, 2, 63, -64, 64, -65, 8191, -8192, 8192, 2147483647, -2147483648, 4294967296, 9223372036854775807, -9223372036854775808]:
    b = io.BytesIO()
    schemaless_writer(b, "long", n)
    emit(f"long {n} {hexs(b.getvalue())}")

section("primitives")


def prim(kind, value, shown):
    b = io.BytesIO()
    schemaless_writer(b, kind, value)
    emit(f"{kind} {shown} {hexs(b.getvalue())}")


prim("int", 150, "150")
prim("int", -2147483648, "-2147483648")
prim("int", 2147483647, "2147483647")
prim("boolean", True, "true")
prim("boolean", False, "false")
prim("float", 1.5, dump_float(1.5))
prim("float", -0.15625, dump_float(-0.15625))
prim("float", float("inf"), dump_float(float("inf")))
prim("double", 0.1, dump_float(0.1))
prim("double", -0.0, dump_float(-0.0))
prim("double", float("-inf"), dump_float(float("-inf")))
prim("string", "", jstr(""))
prim("string", "foo", jstr("foo"))
prim("string", "héllo ☃ \U0001d11e", jstr("héllo ☃ \U0001d11e"))
prim("string", 'quote " back \\ line \n tab \t ctl \x00', jstr('quote " back \\ line \n tab \t ctl \x00'))
prim("bytes", b"", 'x""')
prim("bytes", b"\x00\xff\x10", 'x"00ff10"')
prim("null", None, "null")

section("schemas")
SCHEMA_FILES = ["spec_test.avsc", "event.avsc", "named_refs.avsc", "namespaces.avsc", "logical.avsc", "linked_list.avsc", "unions.avsc"]
for name in SCHEMA_FILES:
    canonical_lines(name, load(name))

section("containers")
EVENT = load("event.avsc")
EVENT_NAMES = named(EVENT)


def event_records():
    base = {
        "id": 0,
        "flag": True,
        "count": -2147483648,
        "ratio": 0.15625,
        "score": 2.5,
        "blob": b"",
        "name": "",
        "kind": "A",
        "tags": [],
        "attrs": {},
        "hash": b"\x00\x01\x02\x03",
        "maybe": None,
        "either": 7,
        "child": None,
        "nested": [],
        "empty_rec": {},
    }
    records = [base]
    records.append(
        {
            "id": 9223372036854775807,
            "flag": False,
            "count": 2147483647,
            "ratio": float("inf"),
            "score": float("-inf"),
            "blob": bytes(range(256)),
            "name": "héllo ☃ \U0001d11e \"quoted\"",
            "kind": "C",
            "tags": ["a", "bb", "ccc"],
            "attrs": {"one": 1, "two": 2, "neg": -9223372036854775808},
            "hash": b"\xff\xfe\xfd\xfc",
            "maybe": "present",
            "either": "text",
            "child": {"value": 1, "next": {"value": 2, "next": {"value": 3, "next": None}}},
            "nested": [{"x": [1.5, 2.5], "y": []}, {}],
            "empty_rec": {},
        }
    )
    records.append(
        {
            "id": -9223372036854775808,
            "flag": True,
            "count": 0,
            "ratio": -0.0,
            "score": 1e300,
            "blob": b"\x01" * 300,
            "name": "x" * 700,
            "kind": "B",
            "tags": ["t%d" % i for i in range(50)],
            "attrs": {"k%d" % i: i * i for i in range(40)},
            "hash": b"\x10\x20\x30\x40",
            "maybe": "",
            "either": b"\xaa\xbb\xcc\xdd",
            "child": {"value": -5, "next": None},
            "nested": [{"m": [float(i) for i in range(10)]} for _ in range(3)],
            "empty_rec": {},
        }
    )
    for i in range(12):
        records.append(
            {
                "id": i * 1000003,
                "flag": i % 2 == 0,
                "count": i - 6,
                "ratio": i / 8.0,
                "score": i / 3.0,
                "blob": bytes([i]) * i,
                "name": "name-%d" % i,
                "kind": ["A", "B", "C"][i % 3],
                "tags": ["tag"] * (i % 4),
                "attrs": {"v": i},
                "hash": bytes([i, i, i, i]),
                "maybe": None if i % 2 else "m%d" % i,
                "either": i if i % 3 == 0 else ("s%d" % i if i % 3 == 1 else b"\x00\x00\x00" + bytes([i])),
                "child": None if i % 2 else {"value": i, "next": None},
                "nested": [],
                "empty_rec": {},
            }
        )
    return records


CODECS = ["null", "deflate", "snappy", "zstandard", "bzip2", "xz"]
RECORDS = event_records()
parsed_event = parse_schema(EVENT)
for codec in CODECS:
    name = f"event_{codec}.avro"
    with open(path(name), "wb") as f:
        writer(f, parsed_event, RECORDS, codec=codec, sync_interval=400, sync_marker=bytes(range(16)))
    with open(path(name), "rb") as f:
        r = reader(f)
        meta = r.metadata
        got = list(r)
    emit(f"file {name} codec {meta['avro.codec'] if 'avro.codec' in meta else 'null'} count {len(got)}")
    for rec in got:
        emit(dump(rec, EVENT, EVENT_NAMES))

section("logical")
LOGICAL = load("logical.avsc")
LOGICAL_NAMES = named(LOGICAL)
D = decimal.Decimal
logical_records = [
    {
        "d": datetime.date(1970, 1, 1),
        "tm": datetime.time(0, 0, 0),
        "tu": datetime.time(23, 59, 59, 999999),
        "tsm": datetime.datetime(1970, 1, 1, tzinfo=datetime.timezone.utc),
        "tsu": datetime.datetime(2024, 2, 29, 12, 30, 45, 123456, tzinfo=datetime.timezone.utc),
        "tsn": 1234567890123456789,
        "lm": datetime.datetime(1999, 12, 31, 23, 59, 59, 999000),
        "lu": datetime.datetime(1969, 7, 20, 20, 17, 40, 1),
        "ln": -1,
        "dec": D("0.00"),
        "decf": D("-1234.567"),
        "u": "123e4567-e89b-12d3-a456-426614174000",
        "uf": uuid.UUID("00000000-0000-4000-8000-000000000001").bytes,
        "dur": struct.pack("<III", 14, 3, 4000000000),
        "opt": None,
    },
    {
        "d": datetime.date(2024, 12, 25),
        "tm": datetime.time(13, 14, 15, 123000),
        "tu": datetime.time(1, 2, 3, 4),
        "tsm": datetime.datetime(1960, 5, 6, 7, 8, 9, 10000, tzinfo=datetime.timezone.utc),
        "tsu": datetime.datetime(1900, 1, 1, tzinfo=datetime.timezone.utc),
        "tsn": -9223372036854775808,
        "lm": datetime.datetime(2000, 1, 1),
        "lu": datetime.datetime(2100, 1, 1, 0, 0, 0, 999999),
        "ln": 9223372036854775807,
        "dec": D("-99999999.99"),
        "decf": D("999999999.999"),
        "u": "ffffffff-ffff-ffff-ffff-ffffffffffff",
        "uf": uuid.UUID("ffffffff-ffff-ffff-ffff-fffffffffffe").bytes,
        "dur": struct.pack("<III", 0, 0, 0),
        "opt": datetime.datetime(2020, 2, 2, 2, 2, 2, 2, tzinfo=datetime.timezone.utc),
    },
]
parsed_logical = parse_schema(LOGICAL)
with open(path("logical.avro"), "wb") as f:
    writer(f, parsed_logical, logical_records, sync_marker=bytes(range(16, 32)))
with open(path("logical.avro"), "rb") as f:
    got = list(reader(f))
emit(f"file logical.avro codec null count {len(got)}")
for rec in got:
    emit(dump(rec, LOGICAL, LOGICAL_NAMES))

section("resolution")
RES_WRITER = load("res_writer.avsc")
RES_READER = load("res_reader.avsc")
res_records = [
    {
        "id": 7,
        "name": "ada",
        "old_nick": "dropped",
        "kind": "B",
        "score": 0.5,
        "tags": [1, 2, 3],
        "data": "text data",
        "ref": 5,
        "alias_me": 99,
        "inner": {"a": 1, "b": "x"},
    },
    {
        "id": 2147483647,
        "name": "bob",
        "old_nick": "n",
        "kind": "D",
        "score": 0.1,
        "tags": [],
        "data": "",
        "ref": "label",
        "alias_me": -5,
        "inner": {"a": -1, "b": ""},
    },
    {
        "id": -1,
        "name": "cyd",
        "old_nick": "",
        "kind": "A",
        "score": -2.25,
        "tags": [2147483647],
        "data": "é",
        "ref": -3,
        "alias_me": 0,
        "inner": {"a": 0, "b": "y"},
    },
]
with open(path("res_writer.avro"), "wb") as f:
    writer(f, parse_schema(RES_WRITER), res_records, sync_marker=bytes(range(32, 48)))
with open(path("res_writer.avro"), "rb") as f:
    got = list(reader(f, reader_schema=parse_schema(RES_READER)))
res_names = named(RES_READER)
emit(f"file res_writer.avro resolved count {len(got)}")
for rec in got:
    emit(dump(rec, RES_READER, res_names))

section("single")
SPEC = load("spec_test.avsc")
fp = rabin(to_parsing_canonical_form(SPEC).encode())
single = b"\xc3\x01" + fp.to_bytes(8, "little")
body = io.BytesIO()
schemaless_writer(body, SPEC, {"a": 27, "b": "foo"})
single += body.getvalue()
emit(f"single {hexs(single)}")
b2 = io.BytesIO()
schemaless_writer(b2, SPEC, {"a": -1000000, "b": "hé"})
single2 = b"\xc3\x01" + fp.to_bytes(8, "little") + b2.getvalue()
emit(f"single2 {hexs(single2)}")

section("serde")
PERSON = load("person.avsc")
sample = {
    "id": 42,
    "name": "Grace",
    "emails": ["g@example.org", "grace@example.com"],
    "address": {"street": "1 Main St", "zip": None},
    "scores": {"math": 99.5, "art": 0.25},
    "kind": "Admin",
    "avatar": b"\x89PNG",
}
sample2 = {
    "id": -7,
    "name": "",
    "emails": [],
    "address": None,
    "scores": {},
    "kind": "Guest",
    "avatar": b"",
}
for label, rec in (("person1", sample), ("person2", sample2)):
    b = io.BytesIO()
    schemaless_writer(b, PERSON, rec)
    with open(path(f"{label}.bin"), "wb") as f:
        f.write(b.getvalue())
    emit(f"{label} {hexs(b.getvalue())}")

section("api")
for text, scale in [("0.00", 2), ("-1234.567", 3), ("99999999.99", 2), ("-0.5", 1), ("128", 0), ("-128", 0), ("0", 0)]:
    unscaled = int(decimal.Decimal(text).scaleb(scale))
    width = 1
    while not -(1 << (8 * width - 1)) <= unscaled < (1 << (8 * width - 1)):
        width += 1
    raw = unscaled.to_bytes(width, "big", signed=True)
    emit(f"decimal_unscaled {text} {scale} {hexs(raw)}")
    emit(f"decimal_text {hexs(raw)} {scale} {format(decimal.Decimal(unscaled).scaleb(-scale), 'f')}")

for label in ("string", "items", "depth"):
    emit(f"limits {label} TooLarge")

resolved_cases = [
    ("int_to_double", "int", "double", 150, None),
    ("long_to_float", "long", "float", 16777216, None),
    ("string_to_bytes", "string", "bytes", "foo", None),
    ("bytes_to_string", "bytes", "string", b"bar", None),
]
for label, writer_type, reader_type, value, _ in resolved_cases:
    b = io.BytesIO()
    schemaless_writer(b, writer_type, value)
    data = b.getvalue()
    got = schemaless_reader(io.BytesIO(data), writer_type, reader_type)
    shown = dump_float(got) if isinstance(got, float) else (f'x"{hexs(got)}"' if isinstance(got, bytes) else jstr(got))
    emit(f"resolve {label} {hexs(data)} {shown}")

SPEC_V2 = load("spec_test_v2.avsc")
body = io.BytesIO()
schemaless_writer(body, SPEC, {"a": 27, "b": "foo"})
got = schemaless_reader(io.BytesIO(body.getvalue()), SPEC, SPEC_V2)
emit(f"single_resolved {dump(got, SPEC_V2, named(SPEC_V2))}")

emit(f"tovalue {dump(sample, PERSON, named(PERSON))}")
emit(f"fromvalue {sample['id']} {sample['name']} {len(sample['emails'])}")

REJECTS = [
    ("duplicate_name", {"type": "record", "name": "A", "fields": [{"name": "x", "type": {"type": "fixed", "name": "A", "size": 1}}]}),
    ("unknown_type", {"type": "record", "name": "A", "fields": [{"name": "x", "type": "Nope"}]}),
    ("two_arrays", [{"type": "array", "items": "int"}, {"type": "array", "items": "long"}]),
    ("nested_union", [["null", "int"], "string"]),
    ("bad_default", {"type": "record", "name": "A", "fields": [{"name": "x", "type": "int", "default": "s"}]}),
    ("fixed_no_size", {"type": "fixed", "name": "F"}),
    ("bad_name", {"type": "record", "name": "1bad", "fields": []}),
    ("dup_symbols", {"type": "enum", "name": "E", "symbols": ["X", "X"]}),
    ("dup_field", {"type": "record", "name": "A", "fields": [{"name": "x", "type": "int"}, {"name": "x", "type": "int"}]}),
    ("record_no_fields", {"type": "record", "name": "A"}),
    ("array_no_items", {"type": "array"}),
    ("bad_enum_default", {"type": "enum", "name": "E", "symbols": ["X"], "default": "Y"}),
    ("repeated_null", ["null", "null"]),
    ("unknown_logical", {"type": "long", "logicalType": "made-up"}),
    ("plain_record", {"type": "record", "name": "A", "fields": [{"name": "x", "type": ["null", "int"], "default": None}]}),
]
SPEC_REJECTS = {"two_arrays", "nested_union", "bad_name", "dup_field", "record_no_fields", "repeated_null"}
with open(path("rejects.txt"), "w") as f:
    for label, s in REJECTS:
        f.write(f"{label}\t{json.dumps(s)}\n")
for label, s in REJECTS:
    try:
        parse_schema(s)
        verdict = "Schema" if label in SPEC_REJECTS else "accepted"
    except Exception:
        verdict = "Schema"
    emit(f"schema {label} {verdict}")

MISMATCHES = [
    ("string_as_long", "long", "abc"),
    ("int_overflow", "int", 3000000000),
    ("missing_field", SPEC, {"b": "x"}),
    ("fixed_size", {"type": "fixed", "name": "F3", "size": 3}, b"abcd"),
    ("unknown_symbol", {"type": "enum", "name": "E1", "symbols": ["X"]}, "Z"),
    ("union_member", ["null", "string"], 5),
]
for label, schema, value in MISMATCHES:
    if fastavro.validation.validate(value, parse_schema(schema), raise_errors=False):
        raise SystemExit(f"{label} validates")
    emit(f"mismatch {label} Mismatch")

RESOLUTION_FAILS = [
    ("string_to_int", "string", "int", "x"),
    ("record_without_default", SPEC, {"type": "record", "name": "test", "fields": [{"name": "z", "type": "int"}]}, {"a": 1, "b": "x"}),
    ("missing_symbol", {"type": "enum", "name": "E", "symbols": ["X", "Y"]}, {"type": "enum", "name": "E", "symbols": ["X"]}, "Y"),
    ("union_branch", ["null", "string"], ["null", "int"], "s"),
]
for label, writer_schema, reader_schema, value in RESOLUTION_FAILS:
    b = io.BytesIO()
    schemaless_writer(b, writer_schema, value)
    try:
        schemaless_reader(io.BytesIO(b.getvalue()), writer_schema, reader_schema)
    except Exception:
        emit(f"resolution {label} Resolution")
    else:
        raise SystemExit(f"{label} resolves")


def zigzag_in(data, pos):
    shift = 0
    acc = 0
    while True:
        b = data[pos]
        pos += 1
        acc |= (b & 0x7F) << shift
        shift += 7
        if not b & 0x80:
            break
    return (acc >> 1) ^ -(acc & 1), pos


def container_layout(data):
    pos = 4
    while True:
        count, pos = zigzag_in(data, pos)
        if count == 0:
            break
        if count < 0:
            _, pos = zigzag_in(data, pos)
            count = -count
        for _ in range(count * 2):
            size, pos = zigzag_in(data, pos)
            pos += size
    header_end = pos + 16
    blocks = []
    pos = header_end
    while pos < len(data):
        start = pos
        _, pos = zigzag_in(data, pos)
        size, pos = zigzag_in(data, pos)
        payload = pos
        pos += size
        blocks.append((start, payload, pos))
        pos += 16
    return header_end, blocks


section("hostile")
for codec in CODECS:
    name = f"event_{codec}.avro"
    with open(path(name), "rb") as f:
        data = f.read()
    header_end, blocks = container_layout(data)
    emit(f"prefixes {name} accepted {1 + len(blocks)} of {len(data) + 1}")

snappy = bytearray(open(path("event_snappy.avro"), "rb").read())
_, snappy_blocks = container_layout(bytes(snappy))
snappy[snappy_blocks[0][2] - 1] ^= 0x01
with open(path("event_badcrc.avro"), "wb") as f:
    f.write(snappy)
plain = bytearray(open(path("event_null.avro"), "rb").read())
_, plain_blocks = container_layout(bytes(plain))
plain[plain_blocks[1][2] + 3] ^= 0xFF
with open(path("event_badsync.avro"), "wb") as f:
    f.write(plain)
emit("badcrc Checksum")
emit("badsync Invalid")
emit("badmagic Invalid")
emit("unknowncodec Codec")
for label in ("person1", "person2"):
    with open(path(f"{label}.bin"), "rb") as f:
        data = f.read()
    emit(f"prefixes {label}.bin rejected {len(data)} of {len(data)}")
emit("trailing person2.bin Invalid")
emit("badunion Invalid")
emit("badenum Invalid")
emit("hugearray TooLarge")
emit("hugestring TooLarge")
emit("negativelength Invalid")
emit("deep 50 accepted")
emit("deep 300 TooLarge")
emit("deep 100000 TooLarge")
empty_array = {"type": "array", "items": {"type": "record", "name": "E", "fields": []}}
b = io.BytesIO()
schemaless_writer(b, empty_array, [{}] * 1000)
b.seek(0)
emit(f"zeroarray {len(schemaless_reader(b, empty_array))} accepted")
emit("zeroarray 1073741824 TooLarge")
for codec in CODECS:
    name = f"event_{codec}.avro"
    emit(f"flips {name} swept {os.path.getsize(path(name))}")

with open(path("written.txt")) as f:
    OUT.extend(f.read().splitlines())

with open(os.path.join(HERE, "..", "std_avro.out"), "w") as f:
    f.write("\n".join(OUT) + "\n")
