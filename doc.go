/*
Package lzss implements LZSS:8bit compression and decompression.

Format: one flag byte per 8 slots; bit 1 = literal (1 byte), bit 0 = pointer (2 bytes).
Pointer: 12-bit backward offset from current output position, 4-bit length nibble.
Default (MinMatchLength 3): length = nibble+3 -> 3..18 bytes. Use MinMatch2 for nibble+2 -> 2..17.
Sliding window: 4096 bytes; filler 0x20 when offset refers before start of output.
Trailing 4-byte checksum: either unsigned (sum of bytes as uint8) or signed (sum as int8).

Use Decompress(src, outLen, opts) with nil for default (unsigned, strict checksum).
Use DecompressBlock(src, outLen, opts) to decode from the beginning of src and get consumed bytes.
Use DecompressFromReader(r, outLen, opts) to decode one block from a stream without reading to EOF.
Use DecompressNFromReader(r, outLens, opts) to decode multiple blocks with known output sizes.
Use DecompressUntilEOF(r, nextOutLen, opts) when output size is provided by a callback.
Use SignedLenientOptions() for formats that use signed checksum and ignore mismatch.
Set Options.MinMatchLength or CompressOptions.MinMatchLength to MinMatch2 for 2..17 back-ref length.

# Examples

Decompress with default options (unsigned checksum, strict):

	out, err := lzss.Decompress(encoded, expectedLen, nil)
	if err != nil {
		return err
	}

Decompress one block from a byte stream and continue from current stream position:

	out, consumed, err := lzss.DecompressFromReader(r, expectedLen, nil)
	if err != nil {
		return err
	}
	_ = consumed

Decompress multiple blocks from a stream with known output sizes:

	out, consumed, err := lzss.DecompressNFromReader(r, []int{lenA, lenB}, nil)
	if err != nil {
		return err
	}
	_ = consumed
	_ = out

Round-trip compress and decompress:

	enc, err := lzss.Compress(data, nil)
	if err != nil {
		return err
	}
	dec, err := lzss.Decompress(enc, len(data), nil)
	if err != nil {
		return err
	}
	// dec equals data

Decompress with signed checksum and skip verification (lenient):

	opts := lzss.SignedLenientOptions()
	out, err := lzss.Decompress(src, outLen, opts)

Compress and decompress with min match length 2 (back-ref length 2..17):

	copts := &lzss.CompressOptions{SearchLimit: 2048, MinMatchLength: lzss.MinMatch2}
	enc, _ := lzss.Compress(data, copts)
	dopts := &lzss.Options{MinMatchLength: lzss.MinMatch2, VerifyChecksum: true}
	dec, _ := lzss.Decompress(enc, len(data), dopts)
*/
package lzss
