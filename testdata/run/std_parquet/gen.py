"""Writes the Parquet files that main.low reads and prints the lines main.low must print for them.

usage: gen.py > reading.txt

Writes files/*.parquet and fixtures.low beside this script and prints the expected output of the reading, api and rows sections of main.low. std_parquet.out is that text, then the "== writing" section that verify_written.py records for the files std/parquet wrote, then the sections for read errors, checksums, limits, sweeps, reader misuse, write errors and paths, which state the behaviour of std/parquet itself and are reviewed by hand against the Parquet format and the READMEs of parquet-testing.

Files are written with pyarrow with the Arrow schema left out of the footer, then read back with pyarrow and formatted by arrowdump.py. Nothing here reads Low output.

The departures from reading with pyarrow, all for the parquet-testing files, are listed in parquet-testing/README.md.
"""
import hashlib
import subprocess
import os
import sys

import pyarrow as pa
import pyarrow.parquet as pq

sys.dont_write_bytecode = True
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "std_arrow"))
sys.path.insert(0, HERE)

import arrowdump  # noqa: E402
import samples  # noqa: E402

CODECS = {"UNCOMPRESSED": "uncompressed", "SNAPPY": "snappy", "GZIP": "gzip", "BROTLI": "brotli", "UNKNOWN": "lz4", "LZ4": "lz4_raw", "LZ4_RAW": "lz4_raw", "ZSTD": "zstd"}


def stat_text(col, logical, stats, which):
    logical = str(logical)
    physical = col.physical_type
    plain = logical in ("None", "String") or logical.startswith("Int(")
    if not plain:
        return "-"
    value = getattr(stats, which)
    if physical in ("INT32", "INT64"):
        return str(value)
    if physical == "BOOLEAN":
        return "true" if value else "false"
    if physical in ("FLOAT", "DOUBLE"):
        return arrowdump.float_text(value)
    if physical == "BYTE_ARRAY" and logical == "String":
        return arrowdump.jstr(value)
    if physical in ("BYTE_ARRAY", "FIXED_LEN_BYTE_ARRAY"):
        return 'x"' + value.hex() + '"'
    return "-"


def chunk_line(col, logical, stats):
    encodings = ",".join(sorted(col.encodings))
    codec = CODECS[col.compression]
    line = f"  chunk {col.path_in_schema} {codec} [{encodings}] values {col.num_values}"
    if not stats:
        return line
    s = col.statistics if col.is_stats_set else None
    if s is None:
        return line + " stats none"
    nulls = s.null_count if s.has_null_count else -1
    if s.has_min_max:
        return f"{line} nulls {nulls} min {stat_text(col, logical, s, 'min')} max {stat_text(col, logical, s, 'max')}"
    return f"{line} nulls {nulls} range none"


def dump(path, stats=True):
    run = subprocess.run([sys.executable, os.path.abspath(__file__), "--one", "1" if stats else "0", path], capture_output=True, text=True)
    if run.returncode != 0:
        return [f"file {os.path.basename(path)}", "error"]
    return run.stdout.splitlines()


def storage_type(t):
    if isinstance(t, pa.ExtensionType):
        return storage_type(t.storage_type)
    return t


def storage_table(table):
    columns = []
    fields = []
    for i, f in enumerate(table.schema):
        col = table.column(i)
        if isinstance(f.type, pa.ExtensionType):
            col = pa.chunked_array([c.storage for c in col.chunks], type=f.type.storage_type)
            f = pa.field(f.name, f.type.storage_type, nullable=f.nullable)
        columns.append(col)
        fields.append(f)
    return pa.Table.from_arrays(columns, schema=pa.schema(fields, metadata=table.schema.metadata))


GEOARROW = " meta ARROW:extension:metadata={},ARROW:extension:name=geoarrow.wkb"
DOCUMENTED_ROWS = {"empty_data_pages_v1.parquet": [1, 2, 3], "empty_data_pages_v2.parquet": [1, 2, 3]}


def schema_lines(pf):
    lines = [line.replace(GEOARROW, "") for line in arrowdump.schema_lines(pf.schema_arrow)]
    pairs = sorted((k.decode(), v.decode()) for k, v in (pf.metadata.metadata or {}).items() if k != b"ARROW:schema")
    lines[0] = "schema" + (" meta " + ",".join(f"{k}={v}" for k, v in pairs) if pairs else "")
    return lines


def dump_file(path, stats):
    pf = pq.ParquetFile(path, page_checksum_verification=True, arrow_extensions_enabled=False)
    md = pf.metadata
    out = [f"file {os.path.basename(path)}"]
    out.extend(schema_lines(pf))
    out.append(f"rows {md.num_rows} groups {md.num_row_groups} created {md.created_by}")
    batches = []
    for g in range(md.num_row_groups):
        rg = md.row_group(g)
        out.append(f"group {g} rows {rg.num_rows}")
        for c in range(rg.num_columns):
            out.append(chunk_line(rg.column(c), pf.schema.column(c).logical_type, stats))
        table = storage_table(pf.read_row_group(g))
        documented = DOCUMENTED_ROWS.get(os.path.basename(path))
        if documented is not None:
            table = pa.table({"int64": pa.array(documented, type=pa.int64())})
        columns = [table.column(i).combine_chunks() for i in range(table.num_columns)]
        batches.append(pa.RecordBatch.from_arrays(columns, schema=table.schema))
    out.append(f"batches {len(batches)}")
    for b in batches:
        out.extend(arrowdump.batch_lines(b, digest=False))
    return out


def write(directory, name, table, options):
    path = os.path.join(directory, f"{name}.parquet")
    pq.write_table(table, path, store_schema=False, **options)
    return path


def one(stats, path):
    sys.stdout.write("\n".join(dump_file(path, stats == "1")) + "\n")


PROJECTIONS = [
    ("flat_snappy_v2", ["s", "i32", "req"], 0),
    ("nested_v1_snappy", ["m", "li", "st"], 0),
    ("nested_groups", ["ll", "rs"], 2),
    ("flat_groups_v2", ["b", "dec10"], 3),
]
COLUMNS = [("nested_v1_snappy", "ll", 0), ("flat_pages_v1", "s_unique", 0), ("flat_groups", "ts_us", 1)]
SELECTIONS = [("flat_groups", "req", 5, 12), ("flat_groups", "req", -100, -50), ("flat_groups_v2", "u8", 100, 130), ("flat_groups_v2", "i64", -5, 5), ("flat_v1_dict", "i32", 0, 0)]


def open_file(name):
    return pq.ParquetFile(os.path.join(HERE, "files", f"{name}.parquet"), page_checksum_verification=True, arrow_extensions_enabled=False)


def api_lines():
    out = []
    for name, columns, group in PROJECTIONS:
        out.append(f"projection {name} {','.join(columns)} group {group}")
        table = open_file(name).read_row_group(group, columns=columns)
        arrays = [table.column(i).combine_chunks() for i in range(table.num_columns)]
        out.extend(arrowdump.batch_lines(pa.RecordBatch.from_arrays(arrays, schema=table.schema)))
    for name, column, group in COLUMNS:
        table = open_file(name).read_row_group(group, columns=[column])
        out.append(f"column {name} {column} group {group} {arrowdump.array_line(table.column(0).combine_chunks())}")
    for name, column, low, high in SELECTIONS:
        pf = open_file(name)
        index = pf.schema_arrow.get_field_index(column)
        picked = []
        for g in range(pf.metadata.num_row_groups):
            stats = pf.metadata.row_group(g).column(index).statistics
            if stats is None or not stats.has_min_max or not (stats.max < low or stats.min > high):
                picked.append(g)
        out.append(f"select {name} {column} {low} {high} {picked}")
    pf = open_file("flat_groups_v2")
    out.append(f"batches flat_groups_v2 {pf.metadata.num_row_groups}")
    return out


ROWS = [("flat_v1_dict", []), ("nested_v1_snappy", []), ("flat_groups", ["req_s", "i32"])]


def storage(col):
    t = col.type
    if pa.types.is_date32(t) or pa.types.is_time32(t):
        return col.cast(pa.int32())
    if pa.types.is_time64(t) or pa.types.is_timestamp(t):
        return col.cast(pa.int64())
    return col


def value_text(v, t):
    if v is None:
        return "null"
    if pa.types.is_boolean(t):
        return "true" if v else "false"
    if pa.types.is_floating(t):
        return arrowdump.float_text(v)
    if pa.types.is_decimal(t):
        return format(v, "f")
    if pa.types.is_string(t):
        return arrowdump.jstr(v)
    if pa.types.is_binary(t) or pa.types.is_fixed_size_binary(t):
        return 'x"' + v.hex() + '"'
    if pa.types.is_map(t):
        return "map[" + ", ".join(f"{value_text(k, t.key_type)}: {value_text(x, t.item_type)}" for k, x in v) + "]"
    if pa.types.is_list(t):
        return "[" + ", ".join(value_text(x, t.value_type) for x in v) + "]"
    if pa.types.is_struct(t):
        return "{" + ", ".join(value_text(v[t.field(k).name], t.field(k).type) for k in range(t.num_fields)) + "}"
    return str(v)


def rows_lines():
    out = []
    for name, columns in ROWS:
        out.append(f"rows {name} {','.join(columns)}")
        table = open_file(name).read(columns=columns or None)
        cols = [storage(table.column(i).combine_chunks()) for i in range(table.num_columns)]
        for r in range(table.num_rows):
            out.append(f"row {r}: " + " | ".join(value_text(c[r].as_py(), c.type) for c in cols))
    return out


def fixtures_source(groups):
    lines = ["struct Fixture {", "    name: str", "    group: str", "    data: []u8", "}", "", "fn fixtures() -> []Fixture {", "    ["]
    for group, folder, names in groups:
        for name in names:
            lines.append(f'        Fixture {{ name: "{name}", group: "{group}", data: bytes(embed("{folder}/{name}")) }}')
    lines.extend(["    ]", "}", ""])
    return "\n".join(lines)


def main():
    if len(sys.argv) > 1 and sys.argv[1] == "--one":
        one(sys.argv[2], sys.argv[3])
        return
    directory = os.path.join(HERE, "files")
    os.makedirs(directory, exist_ok=True)
    flat = samples.flat_table()
    nested = samples.nested_table()
    paths = []
    for name, options in samples.flat_cases():
        paths.append(write(directory, name, flat, options))
    for name, options in samples.nested_cases():
        paths.append(write(directory, name, nested, options))
    lines = []
    groups = [("files", "files", sorted(os.path.basename(p) for p in paths))]
    for path in sorted(paths):
        lines.extend(dump(path))
    for sub in ["data", "bad_data", "encrypted"]:
        folder = os.path.join(HERE, "parquet-testing", sub)
        names = sorted(os.listdir(folder))
        groups.append((sub, f"parquet-testing/{sub}", names))
        lines.append(f"directory {sub}")
        for name in names:
            lines.extend(dump(os.path.join(folder, name), False))
    lines.append("api")
    lines.extend(api_lines())
    lines.extend(rows_lines())
    with open(os.path.join(HERE, "fixtures.low"), "w") as f:
        f.write(fixtures_source(groups))
    text = "\n".join(lines) + "\n"
    sys.stdout.write(text)
    sys.stderr.write(f"sha256 {hashlib.sha256(text.encode()).hexdigest()}\n")


main()
