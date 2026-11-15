# Apache Arrow integration vectors

The `generated_*.stream`, `generated_*.arrow_file` and `generated_interval.json.gz` files are copied unmodified from
`data/arrow-ipc-stream/integration/1.0.0-littleendian/` of https://github.com/apache/arrow-testing at commit
`5a26d35785ff8761e56d79eebff971f6f737e18c`. `LICENSE.txt` is the repository licence (Apache License 2.0).

`generated_map_non_canonical.arrow_file` names its map fields `some_entries`, `some_key` and `some_value`; the
`.stream` file of the same dataset uses `entries`, `key` and `value`. pyarrow does not expose the names of a map's
children, so the names in the expected output for the file were checked against the bytes of the file.
