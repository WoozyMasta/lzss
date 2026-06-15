// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import (
	"encoding/binary"
	"math"
)

const (
	// matchHashBits is the number of bits used to index match hash buckets.
	matchHashBits = 12
	// matchHashSize is the number of buckets in the match hash table.
	matchHashSize = 1 << matchHashBits
	// matchHashMask maps a match hash to a valid bucket index.
	matchHashMask = matchHashSize - 1
)

// matchFinder indexes previous source positions by short-prefix hash.
type matchFinder struct {
	head  [matchHashSize]int32
	chain [WindowSize]int32
}

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
	if matchFinderInputTooLarge(len(src)) {
		return nil, ErrInputTooLarge
	}

	finder := new(matchFinder)
	i := 0
	for i < len(src) {
		bestLen, bestOff := finder.find(src, i, limit, minMatch)

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
			finder.insertRange(src, i, length, minMatch)
			i += length
		} else {
			flagByte |= 1 << bitCount
			out = append(out, src[i])
			finder.insert(src, i, minMatch)
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

// find returns the longest indexed match and its backward offset for pos.
func (finder *matchFinder) find(src []byte, pos, limit, minMatch int) (int, int) {
	maxLen := min(minMatch+15, len(src)-pos)
	if maxLen < minMatch {
		return 0, 0
	}

	hash := matchHash(src, pos, minMatch)
	candidate := int(finder.head[hash]) - 1
	bestLen := 0
	bestOff := 0

	for candidate >= 0 {
		offset := pos - candidate
		if offset > limit || offset > WindowSize {
			break
		}
		candidateMaxLen := min(maxLen, offset)
		if candidateMaxLen <= bestLen {
			candidate = int(finder.chain[candidate&windowMask]) - 1
			continue
		}

		if bestLen == 0 || src[candidate+bestLen] == src[pos+bestLen] {
			length := matchLength(src[candidate:], src[pos:], candidateMaxLen)
			if length > bestLen {
				bestLen = length
				bestOff = offset
				if bestLen == maxLen {
					break
				}
			}
		}

		candidate = int(finder.chain[candidate&windowMask]) - 1
	}

	return bestLen, bestOff
}

// insertRange indexes consecutive source positions consumed by one token.
func (finder *matchFinder) insertRange(src []byte, pos, length, minMatch int) {
	for end := pos + length; pos < end; pos++ {
		finder.insert(src, pos, minMatch)
	}
}

// insert adds one source position to its hash chain.
func (finder *matchFinder) insert(src []byte, pos, minMatch int) {
	if len(src)-pos < minMatch {
		return
	}

	hash := matchHash(src, pos, minMatch)
	finder.chain[pos&windowMask] = finder.head[hash]
	finder.head[hash] = int32(pos + 1) // #nosec G115 -- source length is checked before match finding
}

// matchHash returns the hash bucket for the minimum match prefix at pos.
func matchHash(src []byte, pos, minMatch int) int {
	hash := uint32(src[pos])<<8 | uint32(src[pos+1])
	if minMatch > MinMatch2 {
		hash = hash*0x1e35a7bd ^ uint32(src[pos+2])
	}
	return int(hash & matchHashMask)
}

// matchLength returns the common prefix length capped at maxLen.
func matchLength(left, right []byte, maxLen int) int {
	length := 0
	for length < maxLen && left[length] == right[length] {
		length++
	}
	return length
}

// matchFinderInputTooLarge reports whether absolute int32 positions would overflow.
func matchFinderInputTooLarge(size int) bool {
	return int64(size) > math.MaxInt32
}
