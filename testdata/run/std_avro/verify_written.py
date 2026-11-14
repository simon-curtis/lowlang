"""Checks the container files that std/avro wrote and records their digests.

Usage, with fastavro, zstandard and python-snappy installed:

    python3 verify_written.py <directory with the files the fixture wrote> written.txt

Each file is read with fastavro and must yield the same values as the file of the same name in this directory (fastavro's own file for the event and logical cases, the generator's sample for person.avro). The digests are of the files std/avro wrote and become the "writing" section of std_avro.out.
"""
import hashlib
import json
import os
import sys

from fastavro import reader

from avrodump import dump, named

HERE = os.path.dirname(os.path.abspath(__file__))


def load(name):
    with open(os.path.join(HERE, name)) as f:
        return json.load(f)


def values(path):
    with open(path, "rb") as f:
        r = reader(f)
        return r.metadata, list(r)


def check(directory, name, schema, expected):
    path = os.path.join(directory, name)
    meta, got = values(path)
    names = named(schema)
    lines = [dump(v, schema, names) for v in got]
    if lines != expected:
        raise SystemExit(f"{name}: values differ from the reference")
    with open(path, "rb") as f:
        data = f.read()
    codec = meta.get("avro.codec", "null")
    print(f"{name}: {len(got)} values, codec {codec}", file=sys.stderr)
    return f"wrote {name} {len(data)} {hashlib.sha256(data).hexdigest()}"


def reference(name, schema):
    _, got = values(os.path.join(HERE, name))
    names = named(schema)
    return [dump(v, schema, names) for v in got]


def main():
    directory, target = sys.argv[1], sys.argv[2]
    out = ["== writing"]
    event = load("event.avsc")
    event_lines = reference("event_null.avro", event)
    for codec in ["null", "deflate", "snappy", "zstandard", "bzip2", "xz"]:
        out.append(check(directory, f"event_{codec}.avro", event, event_lines))
    logical = load("logical.avsc")
    out.append(check(directory, "logical.avro", logical, reference("logical.avro", logical)))
    person = load("person.avsc")
    names = named(person)
    sample = {
        "id": 42,
        "name": "Grace",
        "emails": ["g@example.org", "grace@example.com"],
        "address": {"street": "1 Main St", "zip": None},
        "scores": {"math": 99.5, "art": 0.25},
        "kind": "Admin",
        "avatar": b"\x89PNG",
    }
    out.append(check(directory, "person.avro", person, [dump(sample, person, names)] * 2))
    with open(os.path.join(directory, "person.avro"), "rb") as f:
        meta = reader(f).metadata
    if meta.get("origin") not in (b"fixture", "fixture"):
        raise SystemExit("person.avro: metadata entry lost")
    with open(os.path.join(HERE, target), "w") as f:
        f.write("\n".join(out) + "\n")


main()
