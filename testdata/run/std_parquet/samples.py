"""Sample tables and write options shared by gen.py. Files are written by pyarrow only."""
import datetime
import decimal

import pyarrow as pa

FLAT_ROWS = 37


def nullable(i, every, offset, value):
    return None if i % every == offset else value


def strings():
    return ["", "a", "héllo", "naïve café", "x" * 20, "日本語", "a", "a"]


def flat_table():
    n = FLAT_ROWS
    rows = range(n)
    words = strings()
    i32 = [nullable(i, 7, 3, i * i * 37 - 400) for i in rows]
    i32[0] = -2147483648
    i32[1] = 2147483647
    i64 = [nullable(i, 5, 4, i * 10**12 - 5 * 10**13) for i in rows]
    i64[0] = -(2**63)
    i64[1] = 2**63 - 1
    u64 = [nullable(i, 9, 2, (i * 7919) << 40) for i in rows]
    u64[3] = 2**64 - 1
    fields = [
        ("i32", pa.int32(), i32),
        ("i64", pa.int64(), i64),
        ("i8", pa.int8(), [nullable(i, 6, 1, (i * 13) % 256 - 128) for i in rows]),
        ("i16", pa.int16(), [nullable(i, 8, 5, i * 997 - 18000) for i in rows]),
        ("u8", pa.uint8(), [nullable(i, 10, 7, (i * 7) % 256) for i in rows]),
        ("u16", pa.uint16(), [nullable(i, 4, 0, (i * 1999) % 65536) for i in rows]),
        ("u32", pa.uint32(), [nullable(i, 13, 12, (i * 123456789) % 2**32) for i in rows]),
        ("u64", pa.uint64(), u64),
        ("f32", pa.float32(), [nullable(i, 6, 2, i * 0.5 - 3.25) for i in rows]),
        ("f64", pa.float64(), [nullable(i, 7, 4, i * 1.25e10 - 7.5e-3) for i in rows]),
        ("b", pa.bool_(), [nullable(i, 11, 5, (i // 5) % 2 == 0 or i % 3 == 0) for i in rows]),
        ("s", pa.string(), [nullable(i, 9, 8, words[i % len(words)]) for i in rows]),
        ("s_unique", pa.string(), [nullable(i, 12, 6, f"row-{i}-{'z' * (i % 9)}") for i in rows]),
        ("bin", pa.binary(), [nullable(i, 8, 1, bytes(range(i % 6))) for i in rows]),
        ("fixed", pa.binary(4), [nullable(i, 10, 9, bytes([i, i + 1, i + 2, 255 - i])) for i in rows]),
        ("d32", pa.date32(), [nullable(i, 9, 1, 18000 + i * 3 - 20000 * (i % 2)) for i in rows]),
        ("t32", pa.time32("ms"), [nullable(i, 10, 3, i * 1000 + 7) for i in rows]),
        ("t64", pa.time64("us"), [nullable(i, 10, 4, i * 1000003) for i in rows]),
        ("ts_ms", pa.timestamp("ms"), [nullable(i, 9, 3, 1600000000000 + i * 86400123) for i in rows]),
        ("ts_us", pa.timestamp("us", tz="UTC"), [nullable(i, 9, 4, 1600000000000000 + i * 86400123456) for i in rows]),
        ("ts_ns", pa.timestamp("ns"), [nullable(i, 9, 5, 1600000000000000000 + i * 86400123456789) for i in rows]),
        ("dec10", pa.decimal128(10, 2), [nullable(i, 8, 2, decimal.Decimal(i * 12345 - 99999).scaleb(-2)) for i in rows]),
        ("dec38", pa.decimal128(38, 10), [nullable(i, 8, 3, decimal.Decimal((-1) ** i * (10**27 + i)).scaleb(-10)) for i in rows]),
        ("f16", pa.float16(), [nullable(i, 7, 1, i * 0.25 - 2.0) for i in rows]),
    ]
    arrays = []
    schema_fields = []
    for name, t, values in fields:
        arrays.append(pa.array(values, type=t))
        schema_fields.append(pa.field(name, t, nullable=True))
    arrays.append(pa.array([i - 10 for i in rows], type=pa.int32()))
    schema_fields.append(pa.field("req", pa.int32(), nullable=False))
    arrays.append(pa.array([f"r{i % 4}" for i in rows], type=pa.string()))
    schema_fields.append(pa.field("req_s", pa.string(), nullable=False))
    meta = {b"owner": b"sample", b"note": "nøte".encode()}
    return pa.Table.from_arrays(arrays, schema=pa.schema(schema_fields, metadata=meta))


def nested_table():
    n = 14
    li = [[1, 2, 3], None, [], [None], [4, None, 6], [7], [], None, [8, 9], [None, None], [10], [11, 12, 13, 14], [], [15]]
    ls = [["a", "b"], [], None, ["", None], ["héllo"], None, ["x", "y", "z"], [], ["q"], ["a", "a", "a"], [None], ["w"], ["e", "f"], None]
    ll = [[[1, 2], [3]], [], None, [[]], [[None], [4, 5, 6]], [None], [[7], None, [8]], [[]], [[9, 10]], None, [[11], [12], [13]], [], [[None, None]], [[14, 15], []]]
    st = [{"a": 1, "b": "one", "c": {"x": 1.5}}, None, {"a": None, "b": None, "c": None}, {"a": 4, "b": "four", "c": {"x": None}}, {"a": 5, "b": None, "c": {"x": 5.5}}, None, {"a": 7, "b": "seven", "c": None}, {"a": 8, "b": "", "c": {"x": -8.25}}, {"a": None, "b": "nine", "c": {"x": 9.0}}, None, {"a": 11, "b": "eleven", "c": None}, {"a": 12, "b": None, "c": {"x": 12.5}}, {"a": 13, "b": "thirteen", "c": {"x": 13.5}}, None]
    lst = [[{"k": 1, "v": "a"}, {"k": 2, "v": None}], None, [], [{"k": None, "v": "z"}], [None], [{"k": 6, "v": "six"}], [], [{"k": 8, "v": "eight"}, {"k": 9, "v": "nine"}, {"k": 10, "v": "ten"}], None, [{"k": 12, "v": "twelve"}], [], [None, None], [{"k": 14, "v": None}], [{"k": 15, "v": "fifteen"}]]
    m = [[("a", 1), ("b", 2)], None, [], [("c", None)], [("d", 4)], None, [("e", 5), ("f", 6), ("g", 7)], [], [("h", 8)], None, [("i", 9)], [("j", None), ("k", 11)], [], [("l", 12)]]
    m2 = [[(1, ["a", "b"]), (2, [])], None, [], [(3, None)], [(4, ["c"])], None, [(5, ["d", None])], [], [(6, ["e"])], None, [(7, ["f", "g", "h"])], [(8, None)], [], [(9, [])]]
    sl = [{"l": [1, 2], "m": [("a", 1)]}, None, {"l": None, "m": None}, {"l": [], "m": []}, {"l": [None, 3], "m": [("b", None)]}, None, {"l": [4], "m": [("c", 2), ("d", 3)]}, {"l": [5, 6], "m": None}, {"l": None, "m": [("e", 5)]}, None, {"l": [7], "m": []}, {"l": [], "m": [("f", 6)]}, {"l": [8, 9], "m": [("g", 7)]}, None]
    rs = [{"a": i, "b": [i, i + 1]} for i in range(n)]
    t_st = pa.struct([pa.field("a", pa.int32()), pa.field("b", pa.string()), pa.field("c", pa.struct([pa.field("x", pa.float64())]))])
    t_kv = pa.struct([pa.field("k", pa.int32()), pa.field("v", pa.string())])
    t_sl = pa.struct([pa.field("l", pa.list_(pa.int32())), pa.field("m", pa.map_(pa.string(), pa.int32()))])
    t_rs = pa.struct([pa.field("a", pa.int64(), nullable=False), pa.field("b", pa.list_(pa.int64()), nullable=False)])
    columns = [
        ("li", pa.list_(pa.int32()), li, True),
        ("ls", pa.list_(pa.string()), ls, True),
        ("ll", pa.list_(pa.list_(pa.int32())), ll, True),
        ("st", t_st, st, True),
        ("lst", pa.list_(t_kv), lst, True),
        ("m", pa.map_(pa.string(), pa.int64()), m, True),
        ("m2", pa.map_(pa.int32(), pa.list_(pa.string())), m2, True),
        ("sl", t_sl, sl, True),
        ("rs", t_rs, rs, False),
    ]
    arrays = [pa.array(v, type=t) for _, t, v, _ in columns]
    fields = [pa.field(name, t, nullable=ok) for name, t, _, ok in columns]
    return pa.Table.from_arrays(arrays, schema=pa.schema(fields))


def flat_cases():
    ints = ["i32", "i64", "i8", "i16", "u8", "u16", "u32", "u64", "d32", "t32", "t64", "ts_ms", "ts_us", "ts_ns", "req"]
    delta = {name: "DELTA_BINARY_PACKED" for name in ints}
    delta["s"] = "DELTA_LENGTH_BYTE_ARRAY"
    delta["s_unique"] = "DELTA_BYTE_ARRAY"
    delta["bin"] = "DELTA_LENGTH_BYTE_ARRAY"
    delta["req_s"] = "DELTA_BYTE_ARRAY"
    delta["fixed"] = "DELTA_BYTE_ARRAY"
    delta["b"] = "RLE"
    split = {name: "BYTE_STREAM_SPLIT" for name in ["f32", "f64", "i32", "i64", "fixed", "dec10", "f16"]}
    return [
        ("flat_v1_dict", dict(compression="none", data_page_version="1.0")),
        ("flat_v1_plain", dict(compression="none", use_dictionary=False, data_page_version="1.0")),
        ("flat_v2_dict", dict(compression="none", data_page_version="2.0")),
        ("flat_v2_plain", dict(compression="none", use_dictionary=False, data_page_version="2.0")),
        ("flat_snappy_v1", dict(compression="snappy", data_page_version="1.0")),
        ("flat_snappy_v2", dict(compression="snappy", data_page_version="2.0")),
        ("flat_gzip_v1", dict(compression="gzip", data_page_version="1.0")),
        ("flat_gzip_v2", dict(compression="gzip", data_page_version="2.0")),
        ("flat_zstd_v1", dict(compression="zstd", data_page_version="1.0")),
        ("flat_zstd_v2", dict(compression="zstd", data_page_version="2.0")),
        ("flat_brotli_v1", dict(compression="brotli", data_page_version="1.0")),
        ("flat_brotli_v2", dict(compression="brotli", data_page_version="2.0")),
        ("flat_lz4_v1", dict(compression="lz4", data_page_version="1.0")),
        ("flat_lz4_v2", dict(compression="lz4", data_page_version="2.0")),
        ("flat_pages_v1", dict(compression="snappy", data_page_version="1.0", data_page_size=96, write_batch_size=5)),
        ("flat_pages_v2", dict(compression="zstd", data_page_version="2.0", data_page_size=96, write_batch_size=5)),
        ("flat_pages_plain", dict(compression="none", use_dictionary=False, data_page_version="2.0", data_page_size=96, write_batch_size=5)),
        ("flat_delta_v1", dict(compression="none", use_dictionary=False, data_page_version="1.0", column_encoding=delta)),
        ("flat_delta_v2", dict(compression="snappy", use_dictionary=False, data_page_version="2.0", column_encoding=delta)),
        ("flat_delta_pages", dict(compression="none", use_dictionary=False, data_page_version="2.0", column_encoding=delta, data_page_size=96, write_batch_size=5)),
        ("flat_split", dict(compression="none", use_dictionary=False, data_page_version="2.0", column_encoding=split)),
        ("flat_split_pages", dict(compression="zstd", use_dictionary=False, data_page_version="1.0", column_encoding=split, data_page_size=96, write_batch_size=5)),
        ("flat_groups", dict(compression="snappy", row_group_size=10)),
        ("flat_groups_v2", dict(compression="zstd", row_group_size=8, data_page_version="2.0")),
        ("flat_crc_v1", dict(compression="snappy", data_page_version="1.0", write_page_checksum=True)),
        ("flat_crc_v2", dict(compression="gzip", data_page_version="2.0", write_page_checksum=True)),
        ("flat_nostats", dict(compression="none", write_statistics=False)),
        ("flat_int96", dict(compression="snappy", use_deprecated_int96_timestamps=True)),
        ("flat_version1", dict(compression="none", version="1.0", coerce_timestamps="us", allow_truncated_timestamps=True)),
        ("flat_dict_limit", dict(compression="snappy", dictionary_pagesize_limit=40, data_page_size=96, write_batch_size=5)),
    ]


def nested_cases():
    return [
        ("nested_v1_none", dict(compression="none", data_page_version="1.0")),
        ("nested_v1_snappy", dict(compression="snappy", data_page_version="1.0")),
        ("nested_v2_zstd", dict(compression="zstd", data_page_version="2.0")),
        ("nested_v2_plain", dict(compression="none", use_dictionary=False, data_page_version="2.0")),
        ("nested_gzip", dict(compression="gzip", data_page_version="1.0", write_page_checksum=True)),
        ("nested_pages", dict(compression="snappy", data_page_version="2.0", data_page_size=64, write_batch_size=3)),
        ("nested_pages_v1", dict(compression="lz4", data_page_version="1.0", data_page_size=64, write_batch_size=3)),
        ("nested_groups", dict(compression="brotli", row_group_size=5)),
        ("nested_groups_v2", dict(compression="snappy", row_group_size=4, data_page_version="2.0", data_page_size=64, write_batch_size=2)),
        ("nested_legacy_names", dict(compression="none", use_compliant_nested_type=False)),
    ]
