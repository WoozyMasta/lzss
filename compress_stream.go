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
	// windowMask enables faster modulo for 4096-size ring buffer.
	windowMask = WindowSize - 1
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
		if n > 0 {
			source.pos = 0
			source.n = n
		} else if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, false, nil
			}

			return 0, false, err
		} else {
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

	searchLimit := opts.SearchLimit
	if searchLimit > WindowSize {
		searchLimit = WindowSize
	}

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
	historyLen := 0

	// advance commits n bytes from lookahead into history and checksum.
	advance := func(n int) {
		for i := 0; i < n; i++ {
			b := lookahead[i]
			addChecksum(b)

			history[historyPos] = b
			historyPos = (historyPos + 1) & windowMask
			if historyLen < WindowSize {
				historyLen++
			}
		}

		copy(lookahead, lookahead[n:])
		lookahead = lookahead[:len(lookahead)-n]
	}

	// findBestMatch finds longest previous match for current lookahead.
	findBestMatch := func() (int, int) {
		if searchLimit <= 0 {
			return 0, 0
		}

		bestLen := 0
		bestOff := 0
		maxCheck := min(historyLen, searchLimit)
		lookaheadLen := len(lookahead)
		if maxCheck < minMatch {
			return 0, 0
		}

		for off := 1; off <= maxCheck; off++ {
			maxLen := min(MaxMatch, lookaheadLen)
			maxLen = min(maxLen, off)
			if maxLen <= bestLen {
				continue
			}

			if bestLen > 0 {
				probeIdx := (historyPos - off + bestLen) & windowMask
				if history[probeIdx] != lookahead[bestLen] {
					continue
				}
			}

			length := 0
			for length < maxLen {
				idx := (historyPos - off + length) & windowMask
				if history[idx] != lookahead[length] {
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

		return bestLen, bestOff
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
