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

## Unreleased

### Added

* `DecompressBlock(src, outLen, opts)` returns decompressed data
  and consumed byte count for the first LZSS block in a byte slice.
* `DecompressFromReader(r, outLen, opts)` decodes exactly one LZSS block
  from a stream and returns consumed byte count without reading to EOF.

### Changed

* `Decompress(src, outLen, opts)` now validates that the whole input belongs to one block
  and returns `ErrTrailingData` when extra bytes are present.

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
