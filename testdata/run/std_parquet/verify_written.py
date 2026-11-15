"""Checks the Parquet files that std/parquet wrote and records their digests.

Usage, with pyarrow 26.0.1 installed:

    PARQUET_WRITTEN=<directory> low run testdata/run/std_parquet > /dev/null
    python3 verify_written.py <directory> written.txt

Each file is opened with pyarrow with page checksum verification on and must hold the values of the file it was written from: the pyarrow-written file of this directory for flat and nested, the literal rows in BUILT for built. Column chunk statistics must equal those pyarrow wrote for the source whenever the variant writes statistics and one row group, the codec, row group count and encodings must match the variant, and `nostats` files must carry none. The digests are of the files std/parquet wrote and become the "writing" section of std_parquet.out.
"""
import datetime
import decimal
import hashlib
import math
import os
import sys

import pyarrow as pa
import pyarrow.parquet as pq

sys.dont_write_bytecode = True
HERE = os.path.dirname(os.path.abspath(__file__))

SOURCES = {"flat": "flat_v1_dict.parquet", "nested": "nested_v1_snappy.parquet"}
CODECS = {
    "default": "SNAPPY", "plain": "UNCOMPRESSED", "v2_zstd_crc": "ZSTD", "gzip": "GZIP", "brotli_v2": "BROTLI", "lz4_v2": "LZ4",
    "pages_v1": "SNAPPY", "pages_v2": "ZSTD", "groups": "SNAPPY", "groups_v2": "SNAPPY", "dict_limit": "UNCOMPRESSED", "nostats": "SNAPPY",
    "delta_v1": "UNCOMPRESSED", "delta_v2": "SNAPPY", "delta_pages": "UNCOMPRESSED", "split": "UNCOMPRESSED", "split_pages": "ZSTD", "rows": "SNAPPY",
}
GROUPS = {"groups": 8, "groups_v2": 5}
FLAT_ROWS = 37
DELTA = {
    "i32": "DELTA_BINARY_PACKED", "i64": "DELTA_BINARY_PACKED", "u8": "DELTA_BINARY_PACKED", "ts_ns": "DELTA_BINARY_PACKED",
    "s": "DELTA_LENGTH_BYTE_ARRAY", "bin": "DELTA_LENGTH_BYTE_ARRAY", "s_unique": "DELTA_BYTE_ARRAY", "req_s": "DELTA_BYTE_ARRAY",
}
SPLIT = ["f32", "f64", "i32", "i64", "fixed", "dec10", "f16"]
UTC = datetime.timezone.utc

BUILT = {
    "b": [True, None, False],
    "i": [1, None, -3],
    "u16": [0, 65535, None],
    "u64": [2**64 - 1, None, 1],
    "f16": [1.5, None, 65504.0],
    "f32": [1.5, None, math.inf],
    "f64": [1.5, None, -0.0],
    "s": ["a", None, "héllo"],
    "ls": ["a", None, "héllo"],
    "bin": [b"\x00\xff", None, b""],
    "fsb3": [b"abc", None, b"xyz"],
    "dec": [decimal.Decimal("-12.500"), None, decimal.Decimal("0.001")],
    "date": [datetime.date(1970, 1, 1) + datetime.timedelta(days=19000), None, datetime.date(1969, 12, 31)],
    "t64": [datetime.time(1, 0, 0, 1), None, datetime.time(0)],
    "ts": [datetime.datetime(1970, 1, 1, tzinfo=UTC) + datetime.timedelta(seconds=1700), None, datetime.datetime(1969, 12, 31, 23, 59, 59, 999999, tzinfo=UTC)],
    "nulls": [None, None, None],
    "l": [[1, None], None, []],
    "lg": [[], ["p", "q"], None],
    "fsl": [[1, 2], None, [3, 4]],
    "st": [{"x": 1, "y": "p"}, None, {"x": None, "y": "q"}],
    "map": [[("a", 1), ("b", None)], None, []],
    "d": ["a", "b", None],
    "di": [500, 500, -7],
    "req": [10, 20, 30],
    "rst": [{"z": 0.5}, {"z": -2.5}, {"z": 8.0}],
}
BUILT_LOGICAL = {
    "f16": "Float16", "dec": "Decimal(precision=10, scale=3)", "date": "Date", "u16": "Int(bitWidth=16, isSigned=false)", "u64": "Int(bitWidth=64, isSigned=false)",
    "t64": "Time(isAdjustedToUTC=true, timeUnit=microseconds)",
    "ts": "Timestamp(isAdjustedToUTC=true, timeUnit=microseconds, is_from_converted_type=false, force_set_converted_type=false)",
    "s": "String", "ls": "String", "nulls": "Null",
}


def same(a, b):
    if isinstance(a, float) and isinstance(b, float):
        return (a == b and math.copysign(1, a) == math.copysign(1, b)) or (a != a and b != b)
    if isinstance(a, list) and isinstance(b, list):
        return len(a) == len(b) and all(same(x, y) for x, y in zip(a, b))
    if isinstance(a, tuple) and isinstance(b, tuple):
        return len(a) == len(b) and all(same(x, y) for x, y in zip(a, b))
    if isinstance(a, dict) and isinstance(b, dict):
        return a.keys() == b.keys() and all(same(a[k], b[k]) for k in a)
    return type(a) is type(b) and a == b


def rows_of(table):
    return {name: table.column(name).to_pylist() for name in table.column_names}


def open_checked(path):
    return pq.ParquetFile(path, page_checksum_verification=True, arrow_extensions_enabled=False)


def stats_tuple(pf, g, c):
    col = pf.metadata.row_group(g).column(c)
    st = col.statistics
    if st is None:
        return None
    skip = "Timestamp" in str(pf.schema.column(c).logical_type)
    low = st.min if st.has_min_max and not skip else None
    high = st.max if st.has_min_max and not skip else None
    return (st.has_min_max, low, high, st.null_count)


def check_stats(name, pf, source):
    if pf.metadata.num_row_groups != source.metadata.num_row_groups:
        return
    for g in range(pf.metadata.num_row_groups):
        for c in range(pf.metadata.num_columns):
            got = stats_tuple(pf, g, c)
            want = stats_tuple(source, g, c)
            if got != want:
                raise SystemExit(f"{name}: statistics of column {c} differ: {got} against {want}")


def check_variant(name, source_name, variant, pf):
    md = pf.metadata
    for g in range(md.num_row_groups):
        for c in range(md.num_columns):
            col = md.row_group(g).column(c)
            if col.compression != CODECS[variant]:
                raise SystemExit(f"{name}: codec {col.compression}")
            if variant == "nostats" and col.is_stats_set:
                raise SystemExit(f"{name}: statistics were written")
            if variant != "nostats" and not col.is_stats_set:
                raise SystemExit(f"{name}: no statistics for {col.path_in_schema}")
            if source_name == "flat" and variant.startswith("delta") and col.path_in_schema in DELTA and DELTA[col.path_in_schema] not in col.encodings:
                raise SystemExit(f"{name}: {col.path_in_schema} lacks {DELTA[col.path_in_schema]}: {col.encodings}")
            if source_name == "flat" and variant.startswith("split") and col.path_in_schema in SPLIT and "BYTE_STREAM_SPLIT" not in col.encodings:
                raise SystemExit(f"{name}: {col.path_in_schema} lacks BYTE_STREAM_SPLIT: {col.encodings}")
            if variant in ("default", "gzip") and source_name == "flat" and col.path_in_schema == "s" and "RLE_DICTIONARY" not in col.encodings:
                raise SystemExit(f"{name}: s is not dictionary encoded: {col.encodings}")
            if variant == "plain" and "RLE_DICTIONARY" in col.encodings:
                raise SystemExit(f"{name}: dictionary encoded: {col.encodings}")
    if variant in GROUPS:
        rows = FLAT_ROWS if source_name == "flat" else md.num_rows
        want = -(-rows // GROUPS[variant])
        if md.num_row_groups != want:
            raise SystemExit(f"{name}: {md.num_row_groups} row groups, want {want}")
    elif md.num_row_groups != 1:
        raise SystemExit(f"{name}: {md.num_row_groups} row groups")
    if md.created_by != "low std/parquet":
        raise SystemExit(f"{name}: created_by {md.created_by}")


def check_copy(name, source_name, variant, path):
    pf = open_checked(path)
    source = pq.ParquetFile(os.path.join(HERE, "files", SOURCES[source_name]), arrow_extensions_enabled=False)
    got = pf.read()
    want = source.read()
    if not got.schema.equals(want.schema, check_metadata=False):
        raise SystemExit(f"{name}: schema\n{got.schema}\nagainst\n{want.schema}")
    if not got.equals(want):
        raise SystemExit(f"{name}: values differ")
    key_value = {k.decode(): v.decode() for k, v in (pf.metadata.metadata or {}).items()}
    source_meta = {k.decode(): v.decode() for k, v in (source.metadata.metadata or {}).items() if k != b"ARROW:schema"}
    key_value.pop("ARROW:schema", None)
    if key_value != source_meta:
        raise SystemExit(f"{name}: metadata {key_value} against {source_meta}")
    check_variant(name, source_name, variant, pf)
    if variant != "nostats":
        check_stats(name, pf, source)


def check_built(name, variant, path):
    pf = open_checked(path)
    table = pf.read()
    if table.column_names != list(BUILT):
        raise SystemExit(f"{name}: columns {table.column_names}")
    got = rows_of(table)
    for k, want in BUILT.items():
        if not same(got[k], want):
            raise SystemExit(f"{name}: {k} is {got[k]}, want {want}")
    for k, text in BUILT_LOGICAL.items():
        index = [pf.schema.column(c).path for c in range(len(pf.schema))].index(k)
        seen = str(pf.schema.column(index).logical_type)
        if seen != text:
            raise SystemExit(f"{name}: {k} has the logical type {seen}")
    field = pf.schema_arrow.field("i")
    if (field.metadata or {}).get(b"PARQUET:field_id") != b"7":
        raise SystemExit(f"{name}: field id {field.metadata}")
    if pf.schema_arrow.field("req").nullable or pf.schema_arrow.field("rst").nullable:
        raise SystemExit(f"{name}: required fields are nullable")
    meta = {k.decode(): v.decode() for k, v in pf.metadata.metadata.items() if k != b"ARROW:schema"}
    if meta != {"made": "builder", "owner": "low"}:
        raise SystemExit(f"{name}: metadata {meta}")
    check_variant(name, "built", variant, pf)


def main():
    directory, target = sys.argv[1], sys.argv[2]
    out = ["== writing"]
    for source_name in ["flat", "nested", "built"]:
        for variant in CODECS:
            name = f"{source_name}_{variant}.parquet"
            path = os.path.join(directory, name)
            if not os.path.exists(path):
                if source_name != "flat" and variant.startswith(("delta", "split")):
                    continue
                if source_name == "built" and variant == "rows":
                    continue
                raise SystemExit(f"{name}: not written")
            if source_name == "built":
                check_built(name, variant, path)
            else:
                check_copy(name, source_name, variant, path)
            with open(path, "rb") as f:
                data = f.read()
            print(f"{name}: ok, {len(data)} bytes", file=sys.stderr)
            out.append(f"wrote {name} {len(data)} {hashlib.sha256(data).hexdigest()}")
    with open(target, "w") as f:
        f.write("\n".join(out) + "\n")


main()
