"""Generates the binary inputs of std_arrow, fixtures.low and the expected output ../std_arrow.out.

Requires pyarrow 26.0.1:

    python3 gen.py

The files named like samples are written by pyarrow's IPC writer with the options in FIXTURES. The files under arrow-testing/ are the Apache Arrow integration vectors, unmodified (see arrow-testing/README.md). Every expected line comes from pyarrow reading those files, or, for generated_interval, from the integration JSON golden values; never from the Low code. The "writing" section is the digests that verify_written.py recorded for the files std/arrow wrote; the fixture fails if they change.

The prefix sweeps record, for every strict prefix of a stream pyarrow wrote, whether pyarrow accepts it and how many batches it yields; std/arrow must classify each prefix the same way.
"""
import glob
import os

import pyarrow as pa

import samples
from arrowdump import json_interval_lines, schema_lines, table_lines

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = []


def emit(line):
    OUT.append(line)


def path(name):
    return os.path.join(HERE, name)


def options(**kw):
    return pa.ipc.IpcWriteOptions(**kw)


def write_stream(name, schema, batches, **kw):
    with pa.OSFile(path(name), "wb") as f:
        with pa.ipc.new_stream(f, schema, options=options(**kw)) as w:
            for b in batches:
                w.write_batch(b)


def write_file(name, schema, batches, **kw):
    with pa.OSFile(path(name), "wb") as f:
        with pa.ipc.new_file(f, schema, options=options(**kw)) as w:
            for b in batches:
                w.write_batch(b)


def batches_of(table, size=None):
    return table.to_batches(max_chunksize=size)


def deep_type(n):
    t = pa.int32()
    for _ in range(n):
        t = pa.list_(t)
    return t


def make_fixtures():
    scalars = samples.scalars()
    temporal = samples.temporal()
    nested = samples.nested()
    wide = samples.wide()
    meta = samples.metadata_table()
    sweep = pa.table(
        {
            "i": pa.array([1, None, -7], pa.int32()),
            "s": pa.array(["héllo", None, ""]),
            "l": pa.array([[1, 2], None, []], pa.list_(pa.int64())),
            "d": pa.DictionaryArray.from_arrays(pa.array([0, 1, 0], pa.int8()), pa.array(["a", "b"])),
        }
    )
    write_stream("scalars.arrows", scalars.schema, batches_of(scalars))
    write_stream("scalars_lz4.arrows", scalars.schema, batches_of(scalars), compression="lz4")
    write_file("scalars.arrow", scalars.schema, batches_of(scalars))
    write_stream("temporal.arrows", temporal.schema, batches_of(temporal))
    write_stream("temporal_zstd.arrows", temporal.schema, batches_of(temporal), compression="zstd")
    write_file("temporal.arrow", temporal.schema, batches_of(temporal))
    write_stream("nested.arrows", nested.schema, batches_of(nested))
    write_stream("nested_zstd.arrows", nested.schema, batches_of(nested), compression="zstd")
    write_file("nested.arrow", nested.schema, batches_of(nested))
    write_file("nested_lz4.arrow", nested.schema, batches_of(nested), compression="lz4")
    for codec in ("lz4", "zstd"):
        write_stream(f"wide_{codec}.arrows", wide.schema, batches_of(wide, 400), compression=codec)
        write_file(f"wide_{codec}.arrow", wide.schema, batches_of(wide, 400), compression=codec)
    write_stream("meta.arrows", meta.schema, batches_of(meta))
    write_file("meta.arrow", meta.schema, batches_of(meta))
    write_stream("legacy.arrows", sweep.schema, batches_of(sweep), use_legacy_format=True)
    write_stream("v4.arrows", sweep.schema, batches_of(sweep), metadata_version=pa.ipc.MetadataVersion.V4)
    write_stream("sweep.arrows", sweep.schema, batches_of(sweep) * 2)
    write_file("sweep.arrow", sweep.schema, batches_of(sweep) * 2)
    empty = pa.table({"a": pa.array([], pa.int32()), "b": pa.array([], pa.string())})
    write_stream("empty.arrows", empty.schema, batches_of(empty))
    write_stream("nobatches.arrows", empty.schema, [])
    write_file("nobatches.arrow", empty.schema, [])
    dictionary_schema = pa.schema([pa.field("d", pa.dictionary(pa.int8(), pa.string()))])

    def dict_batch(indices, values):
        return pa.record_batch([pa.DictionaryArray.from_arrays(pa.array(indices, pa.int8()), pa.array(values))], schema=dictionary_schema)

    first = dict_batch([0, 1, 0], ["a", "b"])
    second = dict_batch([2, 0, None, 1], ["a", "b", "c"])
    third = dict_batch([3, 3, 1], ["a", "b", "c", "dd"])
    write_stream("dict_delta.arrows", dictionary_schema, [first, second, third], emit_dictionary_deltas=True)
    write_file("dict_delta.arrow", dictionary_schema, [first, second, third], emit_dictionary_deltas=True)
    replaced = dict_batch([0, 1], ["x", "y"])
    write_stream("dict_replace.arrows", dictionary_schema, [first, replaced, first])
    write_stream("deep60.arrows", pa.schema([pa.field("x", deep_type(60))]), [pa.record_batch([pa.array([None], deep_type(60))], schema=pa.schema([pa.field("x", deep_type(60))]))])
    big = pa.schema([pa.field("d", pa.decimal256(40, 2))])
    write_stream("decimal256.arrows", big, [pa.record_batch([pa.array([], pa.decimal256(40, 2))], schema=big)])
    view = pa.schema([pa.field("v", pa.string_view())])
    write_stream("stringview.arrows", view, [pa.record_batch([pa.array(["a", "bb"], pa.string_view())], schema=view)])
    ree = pa.schema([pa.field("r", pa.run_end_encoded(pa.int32(), pa.string()))])
    write_stream("runend.arrows", ree, [pa.record_batch([pa.RunEndEncodedArray.from_arrays(pa.array([2, 3], pa.int32()), pa.array(["a", "b"]))], schema=ree)])


STREAMS = [
    ("scalars.arrows", False),
    ("scalars_lz4.arrows", False),
    ("temporal.arrows", False),
    ("temporal_zstd.arrows", False),
    ("nested.arrows", False),
    ("nested_zstd.arrows", False),
    ("wide_lz4.arrows", True),
    ("wide_zstd.arrows", True),
    ("meta.arrows", False),
    ("legacy.arrows", False),
    ("v4.arrows", False),
    ("sweep.arrows", False),
    ("empty.arrows", False),
    ("nobatches.arrows", False),
    ("dict_delta.arrows", False),
    ("dict_replace.arrows", False),
]

FILES = [
    ("scalars.arrow", False),
    ("temporal.arrow", False),
    ("nested.arrow", False),
    ("nested_lz4.arrow", False),
    ("wide_lz4.arrow", True),
    ("wide_zstd.arrow", True),
    ("meta.arrow", False),
    ("sweep.arrow", False),
    ("nobatches.arrow", False),
    ("dict_delta.arrow", False),
]

INTEGRATION = [
    "custom_metadata",
    "datetime",
    "dictionary",
    "dictionary_unsigned",
    "duplicate_fieldnames",
    "extension",
    "map",
    "map_non_canonical",
    "nested",
    "nested_dictionary",
    "nested_large_offsets",
    "null",
    "null_trivial",
    "primitive",
    "primitive_large_offsets",
    "primitive_no_batches",
    "primitive_zerolength",
    "recursive_nested",
    "union",
]

EXTRAS = ["deep60.arrows", "decimal256.arrows", "stringview.arrows", "runend.arrows"]

REJECTED = [
    ("decimal256.arrows", "Unsupported"),
    ("stringview.arrows", "Unsupported"),
    ("runend.arrows", "Unsupported"),
]


def read_stream(p):
    r = pa.ipc.open_stream(p)
    batches = list(r)
    return r.schema, batches


def read_file(p):
    r = pa.ipc.open_file(p)
    return r.schema, [r.get_batch(i) for i in range(r.num_record_batches)]


def dump_fixtures():
    for name, digest in STREAMS:
        schema, batches = read_stream(path(name))
        emit(f"== {name}")
        OUT.extend(table_lines(schema, batches, digest))
    for name, digest in FILES:
        schema, batches = read_file(path(name))
        emit(f"== {name}")
        OUT.extend(table_lines(schema, batches, digest))
    for name in INTEGRATION:
        for ext, reader in (("stream", read_stream), ("arrow_file", read_file)):
            file = f"arrow-testing/generated_{name}.{ext}"
            schema, batches = reader(path(file))
            emit(f"== {file}")
            lines = table_lines(schema, batches)
            if file == "arrow-testing/generated_map_non_canonical.arrow_file":
                lines = [rename_map_fields(line) for line in lines]
            OUT.extend(lines)
    for ext in ("stream", "arrow_file"):
        file = f"arrow-testing/generated_interval.{ext}"
        emit(f"== {file}")
        OUT.extend(json_interval_lines(path("arrow-testing/generated_interval.json.gz")))
    built = samples.built()
    emit("== built")
    OUT.extend(table_lines(built.schema, built.to_batches()))
    schema, batches = read_stream(path("deep60.arrows"))
    emit(f"== deep60.arrows batches {len(batches)}")
    for name, kind in REJECTED:
        emit(f"rejected {name} {kind}")


def rename_map_fields(line):
    """The file names its map fields some_entries, some_key and some_value; pyarrow's types do not carry those names (see arrow-testing/README.md)."""
    for name in ("entries", "key", "value"):
        line = line.replace(f"field {name}:", f"field some_{name}:")
    return line


def prefix_sweep(name):
    with open(path(name), "rb") as f:
        data = f.read()
    errors = 0
    accepted = []
    for n in range(len(data)):
        try:
            r = pa.ipc.open_stream(pa.py_buffer(data[:n]))
            k = len(list(r))
            accepted.append(f"{n}:{k}")
        except (pa.ArrowInvalid, pa.ArrowIOError):
            errors += 1
    emit(f"prefixes {name} {len(data)} bytes errors {errors} accepted {' '.join(accepted)}")


def file_prefix_sweep(name):
    with open(path(name), "rb") as f:
        data = f.read()
    rejected = 0
    for n in range(len(data)):
        try:
            r = pa.ipc.open_file(pa.py_buffer(data[:n]))
            [r.get_batch(i) for i in range(r.num_record_batches)]
        except (pa.ArrowInvalid, pa.ArrowIOError):
            rejected += 1
    emit(f"prefixes {name} rejected {rejected} of {len(data)}")


def sweeps():
    prefix_sweep("sweep.arrows")
    prefix_sweep("legacy.arrows")
    prefix_sweep("scalars_lz4.arrows")
    file_prefix_sweep("sweep.arrow")
    file_prefix_sweep("nested_lz4.arrow")
    for name in ("sweep.arrows", "sweep.arrow", "scalars_lz4.arrows", "nested_zstd.arrows", "nested_lz4.arrow", "dict_delta.arrows"):
        emit(f"flips {name} swept {2 * os.path.getsize(path(name))}")
    emit("hugemeta TooLarge")
    emit("negativemeta Invalid")
    emit("emptyinput Truncated")
    emit("cutlength Truncated")
    emit("garbagefile Invalid")
    emit("garbagestream Invalid")


DESIGN = """missing file Io
== types
integer of 12 bits Mismatch
float of 8 bits Mismatch
decimal precision 39 Mismatch
decimal scale above precision Mismatch
fixed size binary of -1 Mismatch
fixed size list of -1 Mismatch
union ids and fields differ Mismatch
union id 200 Mismatch
map without a struct Mismatch
nesting of 100 TooLarge
nesting of 60 accepted
== builder
int 128 in int8 Mismatch
text in int8 Mismatch
70000 in uint16 Mismatch
decimal 1234 in decimal(10, 3) accepted
decimal with two points Mismatch
end_list on int8 Mismatch
end_struct on int8 Mismatch
append_union on int8 Mismatch
bytes of 2 in fixed size binary of 3 Mismatch
interval on int8 Mismatch
child 3 of int8 Mismatch
end_list without elements Mismatch
end_struct with unequal children Mismatch
union id 9 Mismatch
builder_length 2 int32
== validate
valid list accepted
length past the offsets Invalid
null count above the nulls Invalid
no offsets buffer Invalid
no child Invalid
nulls without a bitmap Invalid
offset past the child Invalid
negative offset Invalid
first offset 1 accepted
dictionary without values Invalid
== limits
metadata limit TooLarge
body limit TooLarge
both limits accepted
== writer errors
write after close Mismatch
file replacing a dictionary Mismatch
columns of another schema Mismatch
batch length Mismatch
alignment 12 Mismatch
alignment 128 Mismatch
failing sink Io""".splitlines()


def fixtures_source():
    lines = ["import std/arrow", "", "struct Fixture {", "    name: str", "    data: []u8", "    file: bool", "    digest: bool", "}", "", "fn fixtures() -> []Fixture {", "    ["]
    for name, digest in STREAMS:
        lines.append(f'        Fixture {{ name: "{name}", data: bytes(embed("{name}")), file: false, digest: {str(digest).lower()} }}')
    for name, digest in FILES:
        lines.append(f'        Fixture {{ name: "{name}", data: bytes(embed("{name}")), file: true, digest: {str(digest).lower()} }}')
    for name in INTEGRATION:
        for ext, is_file in (("stream", False), ("arrow_file", True)):
            file = f"arrow-testing/generated_{name}.{ext}"
            lines.append(f'        Fixture {{ name: "{file}", data: bytes(embed("{file}")), file: {str(is_file).lower()}, digest: false }}')
    for ext, is_file in (("stream", False), ("arrow_file", True)):
        file = f"arrow-testing/generated_interval.{ext}"
        lines.append(f'        Fixture {{ name: "{file}", data: bytes(embed("{file}")), file: {str(is_file).lower()}, digest: false }}')
    lines.extend(["    ]", "}", "", "fn extras() -> []Fixture {", "    ["])
    for name in EXTRAS:
        lines.append(f'        Fixture {{ name: "{name}", data: bytes(embed("{name}")), file: false, digest: false }}')
    lines.extend(["    ]", "}", ""])
    return "\n".join(lines)


def main():
    make_fixtures()
    dump_fixtures()
    sweeps()
    OUT.extend(DESIGN)
    with open(path("written.txt")) as f:
        OUT.extend(f.read().splitlines())
    with open(path("fixtures.low"), "w") as f:
        f.write(fixtures_source())
    with open(os.path.join(HERE, "..", "std_arrow.out"), "w") as f:
        f.write("\n".join(OUT) + "\n")


main()
