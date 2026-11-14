"""Checks the streams and files that std/arrow wrote and records their digests.

Usage, with pyarrow 26.0.1 installed:

    python3 verify_written.py <directory with the files the fixture wrote> written.txt

Each file is opened with pyarrow's IPC reader, in the format its extension names, and must yield the schema, metadata and values of the sample it was written from: the file of that name in this directory, read with pyarrow, or samples.built() for the builder-made table. The digests are of the files std/arrow wrote and become the "writing" section of std_arrow.out.
"""
import hashlib
import os
import sys

import pyarrow as pa

import samples
from arrowdump import table_lines

HERE = os.path.dirname(os.path.abspath(__file__))

SOURCES = [
    ("scalars", "scalars.arrows"),
    ("temporal", "temporal.arrows"),
    ("nested", "nested.arrows"),
    ("meta", "meta.arrows"),
    ("sweep", "sweep.arrows"),
    ("wide", "wide_lz4.arrows"),
    ("delta", "dict_delta.arrows"),
    ("empty", "nobatches.arrows"),
    ("built", None),
]
VARIANTS = ["plain", "lz4", "zstd"]


def expected(source):
    if source is None:
        built = samples.built()
        return table_lines(built.schema, built.to_batches())
    reader = pa.ipc.open_stream(os.path.join(HERE, source))
    return table_lines(reader.schema, list(reader))


def read_stream(path):
    reader = pa.ipc.open_stream(path)
    batches = list(reader)
    for b in batches:
        b.validate(full=True)
    return table_lines(reader.schema, batches)


def read_file(path):
    reader = pa.ipc.open_file(path)
    batches = [reader.get_batch(i) for i in range(reader.num_record_batches)]
    for b in batches:
        b.validate(full=True)
    return table_lines(reader.schema, batches)


def main():
    directory, target = sys.argv[1], sys.argv[2]
    out = ["== writing"]
    for name, source in SOURCES:
        want = expected(source)
        for variant in VARIANTS:
            for ext, read in (("arrows", read_stream), ("arrow", read_file)):
                if name == "delta" and ext == "arrow":
                    continue
                file = f"{name}_{variant}.{ext}"
                path = os.path.join(directory, file)
                got = read(path)
                if got != want:
                    for a, b in zip(got, want):
                        if a != b:
                            print(f"{file}: got  {a[:300]}\n{file}: want {b[:300]}", file=sys.stderr)
                            break
                    raise SystemExit(f"{file}: differs from the sample")
                with open(path, "rb") as f:
                    data = f.read()
                print(f"{file}: ok, {len(data)} bytes", file=sys.stderr)
                out.append(f"wrote {file} {len(data)} {hashlib.sha256(data).hexdigest()}")
    with open(target, "w") as f:
        f.write("\n".join(out) + "\n")


main()
