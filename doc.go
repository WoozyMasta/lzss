// Package lzss implements LZSS:8bit compression and decompression.
//
// Format: one flag byte per 8 slots; bit 1 = literal (1 byte), bit 0 = pointer (2 bytes).
// Pointer: 12-bit backward offset from current output position, 4-bit length → (len&0x0F)+3 (3..18 bytes).
// Sliding window: 4096 bytes; filler 0x20 when offset refers before start of output.
// Trailing 4-byte checksum: either unsigned (sum of bytes as uint8) or signed (sum as int8).
//
// Use Decompress(src, outLen, opts) with nil for default (unsigned, strict checksum).
// Use SignedLenientOptions() for formats that use signed checksum and ignore mismatch.
package lzss
