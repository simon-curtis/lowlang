"""The sample tables of std_arrow, built with pyarrow."""
import decimal

import struct

import pyarrow as pa

D = decimal.Decimal

INF = float("inf")
NAN = float("nan")


def half(values):
    data = b"".join(struct.pack("<e", 0.0 if v is None else v) for v in values)
    validity = 0
    for i, v in enumerate(values):
        if v is not None:
            validity |= 1 << i
    nulls = sum(v is None for v in values)
    bitmap = pa.py_buffer(validity.to_bytes((len(values) + 7) // 8, "little")) if nulls else None
    return pa.Array.from_buffers(pa.float16(), len(values), [bitmap, pa.py_buffer(data)], null_count=nulls)


def scalars():
    n = 6
    f16 = half([1.5, -0.0, None, 65504.0, 5.960464477539063e-08, INF])
    f16b = half([-INF, NAN, 0.0999755859375, 1.0, 2.0, 1024.0])
    columns = [
        ("null_col", pa.nulls(n)),
        ("bool_col", pa.array([True, False, None, True, False, True])),
        ("i8", pa.array([-128, -1, 0, None, 1, 127], pa.int8())),
        ("i16", pa.array([-32768, -1, 0, None, 1, 32767], pa.int16())),
        ("i32", pa.array([-2147483648, -1, 0, None, 1, 2147483647], pa.int32())),
        ("i64", pa.array([-9223372036854775808, -1, 0, None, 1, 9223372036854775807], pa.int64())),
        ("u8", pa.array([0, 1, None, 127, 128, 255], pa.uint8())),
        ("u16", pa.array([0, 1, None, 32767, 32768, 65535], pa.uint16())),
        ("u32", pa.array([0, 1, None, 2147483647, 2147483648, 4294967295], pa.uint32())),
        ("u64", pa.array([0, 1, None, 9223372036854775807, 9223372036854775808, 18446744073709551615], pa.uint64())),
        ("f16", f16),
        ("f16b", f16b),
        ("f32", pa.array([1.5, -0.0, None, INF, -INF, NAN], pa.float32())),
        ("f64", pa.array([5e-324, -1.7976931348623157e308, 0.1, None, 1e-7, 3.141592653589793], pa.float64())),
        ("dec10_3", pa.array([D("-12.500"), D("0.001"), None, D("9999999.999"), D("-9999999.999"), D("0.000")], pa.decimal128(10, 3))),
        ("dec38_0", pa.array([D(0), D(10**38 - 1), None, D(-(10**38 - 1)), D(1), D(-1)], pa.decimal128(38, 0))),
        ("dec5_5", pa.array([D("0.12345"), None, D("-0.00001"), D("0.00000"), D("0.99999"), D("-0.50000")], pa.decimal128(5, 5))),
        ("utf8", pa.array(["héllo", None, "", 'a"b\\c\n\t\u0001', "😀 emoji", "日本語"], pa.string())),
        ("large_utf8", pa.array(["héllo", None, "", 'a"b\\c\n\t\u0001', "😀 emoji", "日本語"], pa.large_string())),
        ("binary", pa.array([b"", None, b"\x00\xff", b"abc", b"\x00\x00\x00", b"\x80"], pa.binary())),
        ("large_binary", pa.array([b"", None, b"\x00\xff", b"abc", b"\x00\x00\x00", b"\x80"], pa.large_binary())),
        ("fsb3", pa.array([b"abc", None, b"\x00\x01\x02", b"xyz", b"\xff\xff\xff", b"   "], pa.binary(3))),
    ]
    return pa.table({name: col for name, col in columns})


def temporal():
    ints = [0, -1, None, 1700000000, 4102444800]
    small = [0, 86399, None, 1, 3600]
    return pa.table(
        {
            "date32": pa.array([0, -1, None, 19000, 2932896], pa.date32()),
            "date64": pa.array([0, 86400000, None, -86400000, 1640995200000], pa.date64()),
            "time32_s": pa.array(small, pa.time32("s")),
            "time32_ms": pa.array([0, 86399999, None, 1, 3600000], pa.time32("ms")),
            "time64_us": pa.array([0, 86399999999, None, 1, 3600000000], pa.time64("us")),
            "time64_ns": pa.array([0, 86399999999999, None, 1, 3600000000000], pa.time64("ns")),
            "ts_s": pa.array(ints, pa.timestamp("s")),
            "ts_ms": pa.array(ints, pa.timestamp("ms")),
            "ts_us": pa.array(ints, pa.timestamp("us")),
            "ts_ns": pa.array(ints, pa.timestamp("ns")),
            "ts_utc": pa.array(ints, pa.timestamp("us", tz="UTC")),
            "ts_paris": pa.array(ints, pa.timestamp("ms", tz="Europe/Paris")),
            "ts_offset": pa.array(ints, pa.timestamp("ns", tz="+05:30")),
            "dur_s": pa.array([0, -5, None, 1, 2**40], pa.duration("s")),
            "dur_ms": pa.array([0, -5, None, 1, 2**40], pa.duration("ms")),
            "dur_us": pa.array([0, -5, None, 1, 2**40], pa.duration("us")),
            "dur_ns": pa.array([0, -5, None, 1, 2**40], pa.duration("ns")),
            "month_day_nano": pa.array([pa.MonthDayNano([0, 0, 0]), pa.MonthDayNano([-1, -2, -3]), None, pa.MonthDayNano([12, 30, 1000000000]), pa.MonthDayNano([2147483647, -2147483648, 9223372036854775807])], pa.month_day_nano_interval()),
        }
    )


def nested():
    entry = pa.struct([pa.field("x", pa.int32()), pa.field("y", pa.list_(pa.string()))])
    dense = pa.UnionArray.from_dense(
        pa.array([5, 7, 5, 7], pa.int8()),
        pa.array([0, 0, 1, 1], pa.int32()),
        [pa.array([1, None], pa.int32()), pa.array(["x", "y"])],
        ["i", "s"],
        [5, 7],
    )
    sparse = pa.UnionArray.from_sparse(
        pa.array([0, 1, 1, 0], pa.int8()),
        [pa.array([10, 20, 30, 40], pa.int32()), pa.array(["a", None, "c", "d"])],
        ["n", "t"],
        [0, 1],
    )
    dictionary = pa.DictionaryArray.from_arrays(pa.array([0, 1, None, 0], pa.int8()), pa.array(["a", "b"]))
    wide_dictionary = pa.DictionaryArray.from_arrays(pa.array([2, 0, 1, None], pa.uint8()), pa.array([-5, 0, 2**62], pa.int64()), ordered=True)
    inner = pa.DictionaryArray.from_arrays(pa.array([0, 1, 1, 0, 2], pa.int16()), pa.array(["u", "v", None]))
    list_dictionary = pa.ListArray.from_arrays(pa.array([0, 2, 2, 5, 5], pa.int32()), inner)
    struct_dictionary = pa.StructArray.from_arrays([dictionary, pa.array([1, 2, 3, 4], pa.int32())], names=["d", "n"])
    return pa.table(
        {
            "list_i32": pa.array([[1, 2, 3], None, [], [None, 4]], pa.list_(pa.int32())),
            "large_list_str": pa.array([["a", "b"], [], None, [None, "ccc"]], pa.large_list(pa.string())),
            "fsl_i8": pa.array([[1, 2, 3], None, [4, 5, 6], [7, None, 9]], pa.list_(pa.int8(), 3)),
            "struct_ab": pa.array([{"a": 1, "b": "x"}, None, {"a": None, "b": "y"}, {"a": 4, "b": None}], pa.struct([pa.field("a", pa.int32()), pa.field("b", pa.string())])),
            "map_si": pa.array([[("a", 1), ("b", 2)], None, [], [("c", None)]], pa.map_(pa.string(), pa.int32())),
            "list_struct": pa.array([[{"x": 1, "y": ["p", "q"]}, None], None, [], [{"x": None, "y": None}, {"x": 5, "y": []}]], pa.list_(entry)),
            "dense": dense,
            "sparse": sparse,
            "dict_str": dictionary,
            "dict_i64": wide_dictionary,
            "list_dict": list_dictionary,
            "struct_dict": struct_dictionary,
            "list_list": pa.array([[[1], [2, 3]], [[]], None, [[None], None]], pa.list_(pa.list_(pa.int64()))),
        }
    )


def wide(rows=1000):
    labels = ["alpha", "beta", "gamma", "delta"]
    return pa.table(
        {
            "id": pa.array(range(rows), pa.int64()),
            "ratio": pa.array([i / 8 for i in range(rows)], pa.float64()),
            "label": pa.array([labels[i % 4] * (1 + i % 3) for i in range(rows)]),
            "flag": pa.array([i % 5 == 0 for i in range(rows)]),
            "kind": pa.DictionaryArray.from_arrays(pa.array([i % 4 for i in range(rows)], pa.int8()), pa.array(labels)),
            "maybe": pa.array([None if i % 7 == 0 else i for i in range(rows)], pa.int32()),
        }
    )


def metadata_table():
    schema = pa.schema(
        [
            pa.field("id", pa.int32(), nullable=False, metadata={"unit": "none", "ünï": "çode"}),
            pa.field("name", pa.string(), metadata={"lang": "en"}),
            pa.field("tags", pa.list_(pa.field("t", pa.string(), nullable=False)), metadata={"k": ""}),
        ],
        metadata={"origin": "fixture", "empty": "", "ключ": "значение"},
    )
    return pa.table({"id": pa.array([1, 2, 3], pa.int32()), "name": pa.array(["a", None, "c"]), "tags": pa.array([["x"], [], ["y", "z"]], pa.list_(pa.field("t", pa.string(), nullable=False)))}, schema=schema)


def built():
    entries = pa.struct([pa.field("key", pa.string(), nullable=False), pa.field("value", pa.int32())])
    dense = pa.UnionArray.from_dense(
        pa.array([5, 7, 5], pa.int8()),
        pa.array([0, 0, 1], pa.int32()),
        [pa.array([1, None], pa.int32()), pa.array(["z"])],
        ["i", "s"],
        [5, 7],
    )
    sparse = pa.UnionArray.from_sparse(
        pa.array([1, 0, 1], pa.int8()),
        [pa.array([None, 20, None], pa.int32()), pa.array(["a", None, None])],
        ["n", "t"],
        [0, 1],
    )
    schema = pa.schema(
        [
            pa.field("b", pa.bool_()),
            pa.field("i", pa.int32(), metadata={"unit": "count"}),
            pa.field("u16", pa.uint16()),
            pa.field("u64", pa.uint64()),
            pa.field("f16", pa.float16()),
            pa.field("f32", pa.float32()),
            pa.field("f64", pa.float64()),
            pa.field("s", pa.string()),
            pa.field("ls", pa.large_string()),
            pa.field("bin", pa.binary()),
            pa.field("fsb3", pa.binary(3)),
            pa.field("dec", pa.decimal128(10, 3)),
            pa.field("date", pa.date32()),
            pa.field("ts", pa.timestamp("us", tz="UTC")),
            pa.field("iv", pa.month_day_nano_interval()),
            pa.field("nulls", pa.null()),
            pa.field("l", pa.list_(pa.field("item", pa.int32()))),
            pa.field("fsl", pa.list_(pa.field("item", pa.int8()), 2)),
            pa.field("st", pa.struct([pa.field("x", pa.int32()), pa.field("y", pa.string())])),
            pa.field("map", pa.map_(pa.field("key", pa.string(), nullable=False), pa.field("value", pa.int32()))),
            pa.field("dense", dense.type),
            pa.field("sparse", sparse.type),
            pa.field("d", pa.dictionary(pa.int8(), pa.string())),
        ],
        metadata={"made": "builder"},
    )
    columns = [
        pa.array([True, None, False]),
        pa.array([1, None, -3], pa.int32()),
        pa.array([0, 65535, None], pa.uint16()),
        pa.array([18446744073709551615, None, 1], pa.uint64()),
        half([1.5, None, 65504.0]),
        pa.array([1.5, None, INF], pa.float32()),
        pa.array([1.5, None, -0.0], pa.float64()),
        pa.array(["a", None, "héllo"]),
        pa.array(["a", None, "héllo"], pa.large_string()),
        pa.array([b"\x00\xff", None, b""], pa.binary()),
        pa.array([b"abc", None, b"xyz"], pa.binary(3)),
        pa.array([D("-12.500"), None, D("0.001")], pa.decimal128(10, 3)),
        pa.array([19000, None, -1], pa.date32()),
        pa.array([1700000000, None, -1], pa.timestamp("us", tz="UTC")),
        pa.array([pa.MonthDayNano([1, 2, 3]), None, pa.MonthDayNano([-1, -2, -3])], pa.month_day_nano_interval()),
        pa.nulls(3),
        pa.array([[1, 2], None, []], pa.list_(pa.field("item", pa.int32()))),
        pa.array([[1, 2], None, [3, 4]], pa.list_(pa.field("item", pa.int8()), 2)),
        pa.array([{"x": 1, "y": "p"}, None, {"x": None, "y": "q"}], pa.struct([pa.field("x", pa.int32()), pa.field("y", pa.string())])),
        pa.array([[("a", 1), ("b", None)], None, []], pa.map_(pa.field("key", pa.string(), nullable=False), pa.field("value", pa.int32()))),
        dense,
        sparse,
        pa.DictionaryArray.from_arrays(pa.array([0, 1, None], pa.int8()), pa.array(["a", "b"])),
    ]
    return pa.table(columns, schema=schema)
