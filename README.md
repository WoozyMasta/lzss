# lzss

**Lempel–Ziv–Storer–Szymanski** (LZSS)
compression and decompression in Go - LZSS:8bit variant.  
This is **not** the classic Okumura/Apple LZSS (ring-buffer index);
it implements the **BI-style** variant:
backward offset from current output position,
8 flag bits per 8 slots, 12-bit offset + 4-bit length,
4096-byte window, trailing 4-byte checksum.
Used in some archive and texture formats.

## Install

```bash
go get github.com/woozymasta/lzss
```

## Usage

### Decompress

default: unsigned checksum, strict verification:

```go
out, err := lzss.Decompress(compressed, expectedLen, nil)
```

decompress one block from `[]byte` and get consumed bytes:

```go
out, consumed, err := lzss.DecompressBlock(src, expectedLen, nil)
```

decompress one block from stream without reading to EOF:

```go
out, consumed, err := lzss.DecompressFromReader(r, expectedLen, nil)
```

Decompress with signed checksum and lenient verification
(no error on checksum mismatch):

```go
out, err := lzss.Decompress(compressed, expectedLen, lzss.SignedLenientOptions())
```

### Compress

default search limit 2048:

```go
out, err := lzss.Compress(data, nil)
```

with options (search limit, checksum mode):

```go
opts := &lzss.CompressOptions{
    Checksum:    lzss.ChecksumUnsigned,
    SearchLimit: 4096,
}
out, err := lzss.Compress(data, opts)
```

## Format details

* **Flag byte**: 8 bits;
  bit = 1 -> literal (1 byte), bit = 0 -> pointer (2 bytes).
* **Pointer**: 12-bit backward offset, 4-bit length -> 3..18 bytes.
  Stored little-endian.
* **Window**: 4096 bytes.
  When offset refers before start of output, filler byte `0x20` is used.
* **Checksum**: 4 bytes at end.
  Either **unsigned** (sum of bytes as uint8) or **signed** (sum as int8).
  Some formats use signed and ignore mismatch - use `SignedLenientOptions()`
  for decompress.

## Peculiarities

* Back-references can **overlap** the write position (offset < length).
  The decoder must copy byte-by-byte in that case, not block-copy.
* Two checksum modes and optional strict/lenient verification;
  choose options to match the stream format
  (e.g. archives vs certain texture formats).
* Compressed block length is not stored in most containers.
  `DecompressFromReader` and `DecompressBlock` stop when output buffer is full,
  then read checksum and return consumed bytes.
