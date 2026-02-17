// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import (
	"encoding/binary"
)

// CompressOptions configures compression (checksum mode and search limit).
type CompressOptions struct {
	// Checksum mode: unsigned or signed.
	Checksum ChecksumMode
	// 0 = literals only; otherwise max backward distance for match search (e.g. 64..4096).
	SearchLimit int
	// MinMatchLength: 3 (default) encodes length 3..18; 2 encodes 2..17. Zero is 3.
	MinMatchLength int
}

// DefaultCompressOptions returns options for default compression (unsigned checksum, search limit 2048).
func DefaultCompressOptions() *CompressOptions {
	return &CompressOptions{
		Checksum:    ChecksumUnsigned,
		SearchLimit: 2048,
	}
}

// Compress compresses src. Options nil means DefaultCompressOptions().
func Compress(src []byte, opts *CompressOptions) ([]byte, error) {
	if opts == nil {
		opts = DefaultCompressOptions()
	}
	if len(src) == 0 {
		return nil, ErrEmptyInput
	}

	signed := opts.Checksum == ChecksumSigned
	var crc int32
	if signed {
		crc = sumSigned(src)
	} else {
		crc = sumUnsigned(src)
	}

	// Pre-allocate: worst case is all literals + flag bytes + 4 crc; slight overestimate.
	bufCap := len(src) + (len(src)+7)/8 + 4 + 64
	out := make([]byte, 0, bufCap)

	var flagByte byte
	bitCount := 0
	flagPos := -1

	writeFlags := func() {
		if flagPos >= 0 {
			out[flagPos] = flagByte
		}
		flagByte = 0
		bitCount = 0
	}
	startChunk := func() {
		flagPos = len(out)
		out = append(out, 0)
	}

	startChunk()
	minMatch := opts.MinMatchLength
	if minMatch == 0 {
		minMatch = MinMatchDefault
	}

	// If search limit is 0, we don't need to search for matches.
	limit := opts.SearchLimit
	if limit <= 0 {
		// Fast path for literals only (no match search window needed).
		for i := range src {
			flagByte |= 1 << bitCount
			out = append(out, src[i])

			bitCount++
			if bitCount == FlagBits {
				writeFlags()
				if i+1 < len(src) {
					startChunk()
				}
			}
		}

		if bitCount > 0 {
			writeFlags()
		}

		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(crc)) // #nosec G115 -- store checksum bit pattern
		out = append(out, buf...)

		return out, nil
	}

	if limit > WindowSize {
		limit = WindowSize
	}

	i := 0
	for i < len(src) {
		bestLen := 0
		bestOff := 0

		// Find longest match in already processed source prefix (search window).
		if limit > 0 {
			maxCheck := min(min(i, WindowSize), limit)

			for off := 1; off <= maxCheck; off++ {
				maxLen := min(MaxMatch, len(src)-i)
				maxLen = min(maxLen, off)

				length := 0
				for length < maxLen {
					if src[i-off+length] != src[i+length] {
						break
					}

					length++
				}

				if length > bestLen {
					bestLen = length
					bestOff = off
					if bestLen == MaxMatch {
						break
					}
				}
			}
		}

		if bestLen >= minMatch {
			// Encode back-reference: LE 16-bit = [offset_lo8, (offset_hi4<<4)|(length-minMatch)]; length minMatch..minMatch+15.
			offset := bestOff
			length := bestLen
			maxEncLen := minMatch + 15
			if length > maxEncLen {
				length = maxEncLen
			}
			low := offset & 0xFF
			hi4 := (offset & 0x0F00) << 4
			pLen := (length - minMatch) << 8
			pointer := uint16(hi4 | low | pLen) // #nosec G115
			out = append(out, byte(pointer&0xFF), byte(pointer>>8))
			i += length
		} else {
			flagByte |= 1 << bitCount
			out = append(out, src[i])
			i++
		}

		bitCount++
		if bitCount == FlagBits {
			writeFlags()
			if i < len(src) {
				startChunk()
			}
		}
	}

	if bitCount > 0 {
		writeFlags()
	}

	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(crc)) // #nosec G115 -- store checksum bit pattern
	out = append(out, buf...)

	return out, nil
}
