package lzss

import (
	"encoding/binary"
	"fmt"
)

// Decompress decompresses src into a new buffer of length outLen.
// Options nil means DefaultOptions (unsigned checksum, strict verification).
func Decompress(src []byte, outLen int, opts *Options) ([]byte, error) {
	if opts == nil {
		opts = DefaultOptions()
	}
	minMatch := opts.MinMatchLength
	if minMatch == 0 {
		minMatch = MinMatchDefault
	}

	if len(src) < 4 {
		return nil, ErrInputTooShort
	}

	crcPos := len(src) - 4
	data := src[:crcPos]
	readCrc := binary.LittleEndian.Uint32(src[crcPos:])

	signed := opts.Checksum == ChecksumSigned
	var calcCrc int32
	out := make([]byte, outLen)
	pos := 0
	inPos := 0

	// Each flag byte controls 8 slots: bit 1 = literal (1 byte), bit 0 = pointer (2 bytes, back-reference).
	for pos < outLen {
		if inPos >= len(data) {
			return nil, ErrUnexpectedEOF
		}
		flagByte := data[inPos]
		inPos++

		for bit := 0; bit < FlagBits; bit++ {
			if pos >= outLen {
				break
			}

			if inPos >= len(data) && pos < outLen {
				return nil, ErrUnexpectedEOFBit
			}

			if (flagByte>>bit)&1 == 1 {
				if inPos >= len(data) {
					break
				}

				b := data[inPos]
				inPos++
				if pos < outLen {
					out[pos] = b
					pos++
					if signed {
						calcCrc += int32(int8(b))
					} else {
						calcCrc += int32(b)
					}
				}
			} else {
				if inPos+1 >= len(data) {
					break
				}

				// Pointer: LE 16-bit = [offset_lo8, (offset_hi4<<4)|(length-minMatch)]; offset is backward from pos.
				pointer := binary.LittleEndian.Uint16(data[inPos : inPos+2])
				inPos += 2
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
					}
					calcCrc += int32(fillCount) * int32(Filler) // #nosec G115 -- fillCount ≤ MaxMatch
					pos += fillCount
					need -= fillCount
					rpos = 0
				}

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
							if signed {
								calcCrc += int32(int8(b))
							} else {
								calcCrc += int32(b)
							}
						}
					} else {
						copy(out[pos:pos+need], out[rpos:rpos+need])
						for k := 0; k < need; k++ {
							b := out[pos+k]
							if signed {
								calcCrc += int32(int8(b))
							} else {
								calcCrc += int32(b)
							}
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
