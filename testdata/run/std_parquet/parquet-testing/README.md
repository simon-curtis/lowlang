# Apache Parquet test files

The files under `data/`, `bad_data/` and `encrypted/` are copied unmodified from https://github.com/apache/parquet-testing
at commit `bcefdfdcc173d90106860c9e5356a10f25c2d3d0`. `LICENSE.txt` is the repository licence (Apache License 2.0).

`data/` holds every `.parquet` file of the repository's `data` directory, `bad_data/` every file of `bad_data`, and
`encrypted/` the two `.parquet.encrypted` files that encrypt columns and the footer or only the columns. Files with
other names (the CSV and JSON companions and the READMEs) are not used.

`main.low` reads each file and prints its schema, row groups, column chunks and values, and the expected text is what
pyarrow 26.0.1 reads from the same file, with these departures:

- `empty_data_pages_v1.parquet` and `empty_data_pages_v2.parquet` hold the rows 1, 2 and 3 by
  `data/empty_data_pages.md` of the repository; pyarrow reads no rows from them, so the values come from that note.
- pyarrow labels a column of an unrecognised logical type with the `geoarrow.wkb` extension name in field metadata;
  the expected text drops that label.
- Statistics are compared only for the files the fixture itself writes with pyarrow, because pyarrow and `std/parquet`
  apply different validity rules to the statistics of third-party files.
- A file that pyarrow rejects prints `error` in the expected text, and the `== read errors` section names the
  `Error` variant `std/parquet` returns for each file in `bad_data/` and `encrypted/`. Both read paths reject every one of
  them except `ARROW-GH-43605.parquet`, which both read; the variants are the module's own classification and differ
  from pyarrow's messages in detail (for `ARROW-GH-41321.parquet` pyarrow stops at an invalid bit width, while the
  module stops first at a page that runs past its column chunk).
- The two encrypted files fail with `Unsupported`; encryption is not implemented.
