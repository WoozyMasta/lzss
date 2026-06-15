// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	// compressReadBufferSize is internal source read chunk size for stream compressor.
	compressReadBufferSize = 4 * 1024
)

// countingWriter writes to base writer and tracks written byte count.
type countingWriter struct {
	base  io.Writer
	count int64
}

// compressByteSource reads bytes from io.Reader via internal chunk buffer.
type compressByteSource struct {
	base  io.Reader
	buf   []byte
	pos   int
	n     int
	count int64
}

// streamMatchFinder indexes bounded stream history by short-prefix hash.
type streamMatchFinder struct {
	head  [matchHashSize]int64
	chain [WindowSize]int64
}

// Write writes p to base writer and increments the byte counter.
func (writer *countingWriter) Write(p []byte) (int, error) {
	n, err := writer.base.Write(p)
	writer.count += int64(n)

	return n, err
}

// readByte reads one byte from source.
// It returns (0, false, nil) on EOF.
func (source *compressByteSource) readByte() (byte, bool, error) {
	if source.pos >= source.n {
		n, err := source.base.Read(source.buf)
		switch {
		case n > 0:
			source.pos = 0
			source.n = n

		case err != nil:
			if errors.Is(err, io.EOF) {
				return 0, false, nil
			}
			return 0, false, err

		default:
			return 0, false, io.ErrNoProgress
		}
	}

	b := source.buf[source.pos]
	source.pos++
	source.count++

	return b, true, nil
}

// CompressToWriter compresses one stream from src into dst using bounded memory.
// It returns consumed input bytes and written compressed bytes (including checksum).
func CompressToWriter(dst io.Writer, src io.Reader, opts *CompressOptions) (int64, int64, error) {
	if dst == nil {
		return 0, 0, ErrNilWriter
	}
	if src == nil {
		return 0, 0, ErrNilReader
	}
	if opts == nil {
		opts = DefaultCompressOptions()
	}

	minMatch := opts.MinMatchLength
	if minMatch == 0 {
		minMatch = MinMatchDefault
	}

	searchLimit := min(opts.SearchLimit, WindowSize)

	signed := opts.Checksum == ChecksumSigned
	var checksum int32
	addChecksum := func(b byte) {
		if signed {
			checksum += signedByteAsInt32(b)
			return
		}

		checksum += int32(b)
	}

	source := &compressByteSource{
		base: src,
		buf:  make([]byte, compressReadBufferSize),
	}
	countingWriter := &countingWriter{base: dst}
	lookahead := make([]byte, 0, MaxMatch)

	// fillLookahead reads until window reaches MaxMatch or source EOF.
	fillLookahead := func() error {
		for len(lookahead) < MaxMatch {
			b, ok, readErr := source.readByte()
			if readErr != nil {
				return readErr
			}
			if !ok {
				return nil
			}

			lookahead = append(lookahead, b)
		}

		return nil
	}

	if err := fillLookahead(); err != nil {
		return source.count, countingWriter.count, err
	}
	if len(lookahead) == 0 {
		return 0, 0, ErrEmptyInput
	}

	history := make([]byte, WindowSize)
	historyPos := 0
	streamPos := int64(0)
	finder := new(streamMatchFinder)

	// advance commits n bytes from lookahead into history and checksum.
	advance := func(n int) {
		for i := range n {
			b := lookahead[i]
			addChecksum(b)

			history[historyPos] = b
			historyPos = (historyPos + 1) & windowMask
		}

		finder.insertRange(lookahead, streamPos, n, minMatch)
		streamPos += int64(n)
		copy(lookahead, lookahead[n:])
		lookahead = lookahead[:len(lookahead)-n]
	}

	// findBestMatch finds longest previous match for current lookahead.
	findBestMatch := func() (int, int) {
		return finder.find(history, historyPos, lookahead, streamPos, searchLimit, minMatch)
	}

	var (
		flagByte byte
		bitCount int
		chunk    = make([]byte, 0, FlagBits*2)
		flagBuf  [1]byte
	)

	// flushChunk writes one pending 8-slot chunk (flags + payload bytes).
	flushChunk := func() error {
		if bitCount == 0 {
			return nil
		}

		flagBuf[0] = flagByte
		if _, writeErr := countingWriter.Write(flagBuf[:]); writeErr != nil {
			return writeErr
		}
		if len(chunk) > 0 {
			if _, writeErr := countingWriter.Write(chunk); writeErr != nil {
				return writeErr
			}
		}

		flagByte = 0
		bitCount = 0
		chunk = chunk[:0]
		return nil
	}

	// emitLiteral appends literal token into current chunk.
	emitLiteral := func(b byte) error {
		flagByte |= 1 << bitCount
		chunk = append(chunk, b)
		bitCount++
		if bitCount == FlagBits {
			return flushChunk()
		}

		return nil
	}

	// emitPointer appends back-reference token into current chunk.
	emitPointer := func(offset int, length int) error {
		maxEncLen := minMatch + 15
		if length > maxEncLen {
			length = maxEncLen
		}

		low := offset & 0xFF
		hi4 := (offset & 0x0F00) << 4
		pLen := (length - minMatch) << 8
		pointer := uint16(hi4 | low | pLen) // #nosec G115
		chunk = append(chunk, byte(pointer&0xFF), byte(pointer>>8))
		bitCount++
		if bitCount == FlagBits {
			return flushChunk()
		}

		return nil
	}

	for len(lookahead) > 0 {
		bestLen, bestOff := findBestMatch()
		if bestLen >= minMatch {
			maxEncLen := minMatch + 15
			if bestLen > maxEncLen {
				bestLen = maxEncLen
			}

			if err := emitPointer(bestOff, bestLen); err != nil {
				return source.count, countingWriter.count, err
			}

			advance(bestLen)
		} else {
			if err := emitLiteral(lookahead[0]); err != nil {
				return source.count, countingWriter.count, err
			}

			advance(1)
		}

		if err := fillLookahead(); err != nil {
			return source.count, countingWriter.count, err
		}
	}

	if err := flushChunk(); err != nil {
		return source.count, countingWriter.count, err
	}

	var checksumBytes [4]byte
	binary.LittleEndian.PutUint32(checksumBytes[:], uint32(checksum)) // #nosec G115 -- store checksum bit pattern
	if _, err := countingWriter.Write(checksumBytes[:]); err != nil {
		return source.count, countingWriter.count, err
	}

	return source.count, countingWriter.count, nil
}

// find returns the longest indexed stream match and its backward offset.
func (finder *streamMatchFinder) find(
	history []byte,
	historyPos int,
	lookahead []byte,
	pos int64,
	limit int,
	minMatch int,
) (int, int) {
	maxLen := min(minMatch+15, len(lookahead))
	if limit <= 0 || maxLen < minMatch {
		return 0, 0
	}

	hash := matchHashPrefix(lookahead, minMatch)
	candidate := finder.head[hash] - 1
	bestLen := 0
	bestOff := 0

	for candidate >= 0 {
		distance := pos - candidate
		if distance > int64(limit) || distance > WindowSize {
			break
		}
		offset := int(distance)
		candidateMaxLen := min(maxLen, offset)
		if candidateMaxLen <= bestLen {
			candidate = finder.chain[int(candidate)&windowMask] - 1
			continue
		}

		if bestLen == 0 || history[(historyPos-offset+bestLen)&windowMask] == lookahead[bestLen] {
			length := streamMatchLength(history, (historyPos-offset)&windowMask, lookahead, candidateMaxLen)
			if length > bestLen {
				bestLen = length
				bestOff = offset
				if bestLen == maxLen {
					break
				}
			}
		}

		candidate = finder.chain[int(candidate)&windowMask] - 1
	}

	return bestLen, bestOff
}

// insertRange indexes consecutive stream positions consumed by one token.
func (finder *streamMatchFinder) insertRange(lookahead []byte, pos int64, length, minMatch int) {
	for index := range length {
		finder.insert(lookahead[index:], pos+int64(index), minMatch)
	}
}

// insert adds one absolute stream position to its hash chain.
func (finder *streamMatchFinder) insert(prefix []byte, pos int64, minMatch int) {
	if len(prefix) < minMatch {
		return
	}

	hash := matchHashPrefix(prefix, minMatch)
	finder.chain[int(pos)&windowMask] = finder.head[hash]
	finder.head[hash] = pos + 1
}

// streamMatchLength returns the common prefix length for stream history and lookahead.
func streamMatchLength(history []byte, historyStart int, lookahead []byte, maxLen int) int {
	firstLen := min(maxLen, len(history)-historyStart)
	length := matchLength(history[historyStart:], lookahead, firstLen)
	if length < firstLen || length == maxLen {
		return length
	}

	return length + matchLength(history, lookahead[length:], maxLen-length)
}
