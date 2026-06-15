// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// streamWriteBufferSize is the chunk size used before flushing decoded bytes to writer.
	streamWriteBufferSize = 32 * 1024
)

// DecompressToWriter decompresses one LZSS block from src into dst without allocating full output.
// It returns consumed compressed byte count (including trailing checksum).
func DecompressToWriter(dst io.Writer, src io.Reader, outLen int, opts *Options) (int64, error) {
	if dst == nil {
		return 0, ErrNilWriter
	}

	countingReader, err := newCountingByteReader(src)
	if err != nil {
		return 0, err
	}

	err = decompressToWriterFromByteReader(dst, countingReader, outLen, opts)
	if err != nil {
		return countingReader.count, err
	}

	return countingReader.count, nil
}

// decompressToWriterFromByteReader decompresses one LZSS block into writer from counted source.
func decompressToWriterFromByteReader(dst io.Writer, r *countingByteReader, outLen int, opts *Options) error {
	if opts == nil {
		opts = DefaultOptions()
	}

	if outLen < 0 {
		return ErrNegativeOutLen
	}

	minMatch := opts.MinMatchLength
	if minMatch == 0 {
		minMatch = MinMatchDefault
	}

	signed := opts.Checksum == ChecksumSigned
	var calcCrc int32
	window := make([]byte, WindowSize)
	windowPos := 0
	produced := 0
	writeBuf := make([]byte, 0, streamWriteBufferSize)

	addChecksum := func(b byte) {
		if signed {
			calcCrc += signedByteAsInt32(b)
			return
		}

		calcCrc += int32(b)
	}

	addChecksumSpan := func(data []byte) {
		if signed {
			calcCrc += sumSignedI32(data)
			return
		}

		calcCrc += sumUnsigned(data)
	}

	emitByte := func(b byte) error {
		window[windowPos] = b
		windowPos = (windowPos + 1) & windowMask
		produced++
		addChecksum(b)
		writeBuf = append(writeBuf, b)
		if len(writeBuf) == cap(writeBuf) {
			if _, err := dst.Write(writeBuf); err != nil {
				return err
			}

			writeBuf = writeBuf[:0]
		}

		return nil
	}

	emitSpan := func(data []byte) error {
		firstLen := min(len(data), WindowSize-windowPos)
		copy(window[windowPos:], data[:firstLen])
		copy(window, data[firstLen:])
		windowPos = (windowPos + len(data)) & windowMask
		produced += len(data)
		addChecksumSpan(data)

		if len(writeBuf)+len(data) > cap(writeBuf) {
			if _, err := dst.Write(writeBuf); err != nil {
				return err
			}
			writeBuf = writeBuf[:0]
		}
		writeBuf = append(writeBuf, data...)
		return nil
	}

	flush := func() error {
		if len(writeBuf) == 0 {
			return nil
		}

		_, err := dst.Write(writeBuf)
		writeBuf = writeBuf[:0]
		return err
	}

	readByte := func(eofErr error) (byte, error) {
		b, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return 0, eofErr
			}

			return 0, err
		}

		return b, nil
	}

	for produced < outLen {
		flagByte, err := readByte(ErrUnexpectedEOF)
		if err != nil {
			return err
		}

		if flagByte == 0xff && outLen-produced >= FlagBits {
			literals := r.scratch[:FlagBits]
			if err := r.readFull(literals); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return ErrUnexpectedEOFBit
				}
				return err
			}
			if err := emitSpan(literals); err != nil {
				return err
			}
			continue
		}

		for bit := range FlagBits {
			if produced >= outLen {
				break
			}

			if (flagByte>>bit)&1 == 1 {
				b, err := readByte(ErrUnexpectedEOFBit)
				if err != nil {
					return err
				}

				if err := emitByte(b); err != nil {
					return err
				}

				continue
			}

			lo, err := readByte(ErrUnexpectedEOFBit)
			if err != nil {
				return err
			}
			hi, err := readByte(ErrUnexpectedEOFBit)
			if err != nil {
				return err
			}

			pointer := uint16(lo) | (uint16(hi) << 8)
			offset := int(pointer&0x00FF) + (int(pointer&0xF000) >> 4)
			length := int((pointer&0x0F00)>>8) + minMatch

			need := min(length, outLen-produced)
			if offset == 0 {
				match := r.scratch[:need]
				clear(match)
				if err := emitSpan(match); err != nil {
					return err
				}
				continue
			}

			match := r.scratch[:need]
			fillerLen := min(max(offset-produced, 0), need)
			for i := range fillerLen {
				match[i] = Filler
			}
			if fillerLen > 0 {
				if err := emitSpan(match[:fillerLen]); err != nil {
					return err
				}
				need -= fillerLen
			}
			if need == 0 {
				continue
			}
			match = match[:need]

			if offset >= need {
				sourcePos := (windowPos - offset) & windowMask
				firstLen := min(need, WindowSize-sourcePos)
				copy(match, window[sourcePos:sourcePos+firstLen])
				copy(match[firstLen:], window[:need-firstLen])
				if err := emitSpan(match); err != nil {
					return err
				}
				continue
			}

			for i := 0; i < need; i++ {
				var b byte
				switch {
				case produced < offset:
					b = Filler
				default:
					index := windowPos - offset
					if index < 0 {
						index += WindowSize
					}

					b = window[index]
				}

				if err := emitByte(b); err != nil {
					return err
				}
			}
		}
	}

	var checksumBytes [4]byte
	for i := range 4 {
		b, err := readByte(ErrInputTooShort)
		if err != nil {
			return err
		}

		checksumBytes[i] = b
	}

	if err := flush(); err != nil {
		return err
	}

	if !opts.VerifyChecksum {
		return nil
	}

	readCrc := binary.LittleEndian.Uint32(checksumBytes[:])
	if signed {
		// #nosec G115 -- intentional: compare stored uint32 as int32 for signed checksum
		if calcCrc != int32(readCrc) {
			return fmt.Errorf("checksum mismatch (signed): got=0x%x expected=0x%x", uint32(calcCrc), readCrc)
		}

		return nil
	}

	// #nosec G115 -- intentional: compare int32 sum as uint32 for unsigned checksum
	if uint32(calcCrc) != readCrc {
		return fmt.Errorf("checksum mismatch (unsigned): got=0x%x expected=0x%x", uint32(calcCrc), readCrc)
	}

	return nil
}
