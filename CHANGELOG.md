# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.1.5][] - 2026-02-18

### Added

* `DecompressToWriter(dst, src, outLen, opts)`
  for bounded-memory stream decode directly into `io.Writer`.
* Decode benchmarks for stream paths.

### Changed

* Added explicit `ErrNilWriter` for stream decode API validation.

[0.1.5]: https://github.com/WoozyMasta/lzss/compare/v0.1.4...v0.1.5

## [0.1.4][] - 2026-02-17

### Changed

* Optimized decode hot path for byte-slice input: `DecompressBlock`
  now uses a dedicated slice fast path instead of `io.ByteReader`.
* Split slice decode into specialized unsigned/signed checksum paths to reduce
  branching in inner loops.
* Improved checksum handling in non-overlap copy path by summing source spans
  directly before `copy(...)`, reducing per-byte loop overhead.
* Reduced allocations in core benchmarks:
  `Compress` and `Decompress` now use `1 alloc/op` on the main benchmark flow.
* Reduced compressor temporary memory usage (`B/op`) by removing duplicated
  reconstructed-buffer state in match search.
* Added benchmark throughput reporting (`MB/s`).

[0.1.4]: https://github.com/WoozyMasta/lzss/compare/v0.1.3...v0.1.4

## [0.1.3][] - 2026-02-13

### Changed

* Refactoring code to reduce cognitive complexity and
  leverage modern programming techniques.

[0.1.3]: https://github.com/WoozyMasta/lzss/compare/v0.1.2...v0.1.3

## [0.1.2][] - 2026-02-11

### Added

* `DecompressBlock(src, outLen, opts)` returns decompressed data
  and consumed byte count for the first LZSS block in a byte slice.
* `DecompressFromReader(r, outLen, opts)` decodes exactly one LZSS block
  from a stream and returns consumed byte count without reading to EOF.
* `DecompressNFromReader(r, outLens, opts)` decodes multiple blocks from
  a stream using expected output lengths.
* `DecompressUntilEOF(r, nextOutLen, opts)` decodes blocks while callback
  returns next expected output size.

### Changed

* `Decompress(src, outLen, opts)` now validates that the whole input belongs
  to one block and returns `ErrTrailingData` when extra bytes are present.

[0.1.2]: https://github.com/WoozyMasta/lzss/compare/v0.1.1...v0.1.2

## [0.1.1][] - 2026-02-11

### Added

* `Options.MinMatchLength` and `CompressOptions.MinMatchLength` for support
  back-ref length 2..17 (MinMatch2) in addition to default 3..18

[0.1.1]: https://github.com/WoozyMasta/lzss/compare/v0.1.0...v0.1.1

## [0.1.0][] - 2026-02-04

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/lzss/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
