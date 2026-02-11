package lzss

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Decompress decompresses src into a new buffer of length outLen.
// Options nil means DefaultOptions (unsigned checksum, strict verification).
func Decompress(src []byte, outLen int, opts *Options) ([]byte, error) {
	if len(src) < 4 {
		return nil, ErrInputTooShort
	}

	out, consumed, err := DecompressBlock(src, outLen, opts)
	if err != nil {
		return nil, err
	}

	if consumed != len(src) {
		return nil, fmt.Errorf("%w: consumed=%d input=%d", ErrTrailingData, consumed, len(src))
	}

	return out, nil
}

// DecompressBlock decompresses one LZSS block from the beginning of src.
// It returns decompressed bytes and the number of consumed bytes (data + checksum).
// Unlike Decompress, this function ignores trailing bytes after the first block.
func DecompressBlock(src []byte, outLen int, opts *Options) ([]byte, int, error) {
	if len(src) < 4 {
		return nil, 0, ErrInputTooShort
	}

	reader := &sliceByteReader{data: src}
	out, err := decompressFromByteReader(reader, outLen, opts)
	if err != nil {
		return nil, reader.pos, err
	}

	return out, reader.pos, nil
}

// DecompressFromReader decompresses one LZSS block from r and returns consumed bytes.
// Decoding stops exactly after outLen output bytes and trailing 4-byte checksum are read.
func DecompressFromReader(r io.Reader, outLen int, opts *Options) ([]byte, int64, error) {
	countingReader, err := newCountingByteReader(r)
	if err != nil {
		return nil, 0, err
	}

	out, err := decompressFromByteReader(countingReader, outLen, opts)
	if err != nil {
		return nil, countingReader.count, err
	}

	return out, countingReader.count, nil
}

// DecompressNFromReader decompresses N LZSS blocks from r with expected output lengths.
// It returns decompressed blocks and total consumed byte count across all blocks.
func DecompressNFromReader(r io.Reader, outLens []int, opts *Options) ([][]byte, int64, error) {
	countingReader, err := newCountingByteReader(r)
	if err != nil {
		return nil, 0, err
	}

	blocks := make([][]byte, 0, len(outLens))
	for i, outLen := range outLens {
		block, decodeErr := decompressFromByteReader(countingReader, outLen, opts)
		if decodeErr != nil {
			return blocks, countingReader.count, fmt.Errorf("decode block %d: %w", i, decodeErr)
		}

		blocks = append(blocks, block)
	}

	return blocks, countingReader.count, nil
}

// DecompressUntilEOF decompresses blocks from r while nextOutLen returns (outLen, true).
// nextOutLen must provide expected unpacked size for each next block.
func DecompressUntilEOF(r io.Reader, nextOutLen func() (int, bool), opts *Options) ([][]byte, int64, error) {
	if nextOutLen == nil {
		return nil, 0, ErrNilOutLenProvider
	}

	countingReader, err := newCountingByteReader(r)
	if err != nil {
		return nil, 0, err
	}

	var blocks [][]byte
	for i := 0; ; i++ {
		outLen, ok := nextOutLen()
		if !ok {
			break
		}

		block, decodeErr := decompressFromByteReader(countingReader, outLen, opts)
		if decodeErr != nil {
			return blocks, countingReader.count, fmt.Errorf("decode block %d: %w", i, decodeErr)
		}

		blocks = append(blocks, block)
	}

	return blocks, countingReader.count, nil
}

func newCountingByteReader(r io.Reader) (*countingByteReader, error) {
	if r == nil {
		return nil, ErrNilReader
	}

	var byteReader io.ByteReader
	if existing, ok := r.(io.ByteReader); ok {
		byteReader = existing
	} else {
		byteReader = bufio.NewReader(r)
	}

	return &countingByteReader{base: byteReader}, nil
}

// decompressFromByteReader decompresses from a byte reader.
func decompressFromByteReader(r io.ByteReader, outLen int, opts *Options) ([]byte, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	if outLen < 0 {
		return nil, ErrNegativeOutLen
	}

	minMatch := opts.MinMatchLength
	if minMatch == 0 {
		minMatch = MinMatchDefault
	}

	signed := opts.Checksum == ChecksumSigned
	var calcCrc int32
	out := make([]byte, outLen)
	pos := 0

	addChecksum := func(b byte) {
		if signed {
			calcCrc += int32(int8(b))
		} else {
			calcCrc += int32(b)
		}
	}

	// Read a byte from the reader.
	// If the reader returns an EOF error, return the error passed as eofErr.
	// Otherwise, return the error from the reader.
	readByte := func(eofErr error) (byte, error) {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, eofErr
			}

			return 0, err
		}

		return b, nil
	}

	// Iterate over output bytes.
	for pos < outLen {
		flagByte, err := readByte(ErrUnexpectedEOF)
		if err != nil {
			return nil, err
		}

		// Iterate over flag bytes for each output byte.
		for bit := 0; bit < FlagBits; bit++ {
			if pos >= outLen {
				break
			}

			// If bit is 1, it's a literal: 1 bit, 1 byte otherwise it's a pointer.
			if (flagByte>>bit)&1 == 1 {
				b, err := readByte(ErrUnexpectedEOFBit)
				if err != nil {
					return nil, err
				}

				out[pos] = b
				addChecksum(b)
				pos++
			} else {
				lo, err := readByte(ErrUnexpectedEOFBit)
				if err != nil {
					return nil, err
				}
				hi, err := readByte(ErrUnexpectedEOFBit)
				if err != nil {
					return nil, err
				}

				// Pointer: LE 16-bit = [offset_lo8, (offset_hi4<<4)|(length-minMatch)]; offset is backward from pos.
				pointer := uint16(lo) | (uint16(hi) << 8)
				low8 := int(pointer & 0xFF)
				hi4 := int((pointer & 0xF000) >> 12)
				offset := low8 + (hi4 << 8)
				length := int((pointer&0x0F00)>>8) + minMatch

				rpos := pos - offset // source start in output buffer
				need := length       // bytes to copy (may be capped by outLen later)

				// Offset can refer before start of output: fill with Filler (0x20) for those bytes.
				if rpos < 0 {
					fillCount := -rpos
					if fillCount > need {
						fillCount = need
					}
					endFill := pos + fillCount
					if endFill > outLen {
						endFill = outLen
					}
					for j := pos; j < endFill; j++ {
						out[j] = Filler
						addChecksum(Filler)
					}
					pos += fillCount
					need -= fillCount
					rpos = 0
				}

				// Copy bytes from source to output.
				if need > 0 && pos < outLen {
					if pos+need > outLen {
						need = outLen - pos
					}
					// Overlapping back-ref (offset < need): must copy byte-by-byte so each written byte
					// is visible to the next read (RLE-like). copy(dst, src) does not handle overlap.
					if offset < need {
						for k := 0; k < need; k++ {
							b := out[rpos+k]
							out[pos+k] = b
							addChecksum(b)
						}
					} else {
						copy(out[pos:pos+need], out[rpos:rpos+need])
						for k := 0; k < need; k++ {
							addChecksum(out[pos+k])
						}
					}
					pos += need
				}
			}

			if pos >= outLen {
				break
			}
		}

		if pos >= outLen {
			break
		}
	}

	checksumBytes := make([]byte, 4)
	for i := 0; i < 4; i++ {
		b, err := readByte(ErrInputTooShort)
		if err != nil {
			return nil, err
		}
		checksumBytes[i] = b
	}
	readCrc := binary.LittleEndian.Uint32(checksumBytes)

	if opts.VerifyChecksum {
		if signed {
			// #nosec G115 -- intentional: compare stored uint32 as int32 for signed checksum
			if calcCrc != int32(readCrc) {
				return nil, fmt.Errorf("checksum mismatch (signed): got=0x%x expected=0x%x", uint32(calcCrc), readCrc)
			}
		} else {
			// #nosec G115 -- intentional: compare int32 sum as uint32 for unsigned checksum
			if uint32(calcCrc) != readCrc {
				return nil, fmt.Errorf("checksum mismatch (unsigned): got=0x%x expected=0x%x", uint32(calcCrc), readCrc)
			}
		}
	}

	return out, nil
}
