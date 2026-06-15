// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"strconv"
	"testing"
)

func TestDecodeGoldenVectors(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    []byte
	}{
		{name: "partial-literal-group", payload: []byte{0x01, 'x'}, want: []byte("x")},
		{name: "non-overlap-backref", payload: []byte{0x07, 'A', 'B', 'C', 3, 0}, want: []byte("ABCABC")},
		{name: "overlap-backref", payload: []byte{0x01, 'A', 1, 15}, want: bytes.Repeat([]byte("A"), 19)},
		{name: "filler-before-output", payload: []byte{0x00, 4, 0}, want: bytes.Repeat([]byte{Filler}, 3)},
		{name: "offset-zero", payload: []byte{0x00, 0, 0}, want: make([]byte, 3)},
		{name: "final-match-is-capped", payload: []byte{0x01, 'A', 1, 15}, want: bytes.Repeat([]byte("A"), 5)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := appendChecksum(test.payload, test.want, ChecksumUnsigned)
			assertDecodePathsEqual(t, test.want, encoded, DefaultOptions())
		})
	}
}

func TestRoundTripOptionMatrix(t *testing.T) {
	inputs := [][]byte{
		[]byte("single option matrix payload"),
		bytes.Repeat([]byte("ab"), 128),
		bytes.Repeat([]byte{0x00, 0x7f, 0x80, 0xff}, 128),
	}

	for _, searchLimit := range []int{0, 1, 64, 256, 2048, 4096} {
		for _, minMatch := range []int{MinMatchDefault, MinMatch2} {
			for _, checksum := range []ChecksumMode{ChecksumUnsigned, ChecksumSigned} {
				compressOpts := &CompressOptions{
					Checksum:       checksum,
					SearchLimit:    searchLimit,
					MinMatchLength: minMatch,
				}
				decompressOpts := &Options{
					Checksum:       checksum,
					VerifyChecksum: true,
					MinMatchLength: minMatch,
				}

				for _, input := range inputs {
					encoded, err := Compress(input, compressOpts)
					if err != nil {
						t.Fatal(err)
					}
					assertDecodePathsEqual(t, input, encoded, decompressOpts)
				}
			}
		}
	}
}

func TestCompressRatioRegression(t *testing.T) {
	longDistanceBlock := make([]byte, 3072)
	_, _ = rand.New(rand.NewSource(1)).Read(longDistanceBlock)

	tests := []struct {
		name     string
		input    []byte
		opts     *CompressOptions
		maxRatio float64
	}{
		{
			name:     "overlap",
			input:    bytes.Repeat([]byte("A"), 4096),
			opts:     DefaultCompressOptions(),
			maxRatio: 0.13,
		},
		{
			name:     "min-match-2",
			input:    bytes.Repeat([]byte("ab"), 2048),
			opts:     &CompressOptions{SearchLimit: 256, MinMatchLength: MinMatch2},
			maxRatio: 0.13,
		},
		{
			name:     "long-distance",
			input:    append(append([]byte(nil), longDistanceBlock...), longDistanceBlock...),
			opts:     &CompressOptions{SearchLimit: 4096},
			maxRatio: 0.65,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := Compress(test.input, test.opts)
			if err != nil {
				t.Fatal(err)
			}
			ratio := float64(len(encoded)) / float64(len(test.input))
			if ratio > test.maxRatio {
				t.Fatalf("ratio=%f max=%f", ratio, test.maxRatio)
			}
		})
	}
}

func TestCompressDeterministicOutput(t *testing.T) {
	input := bytes.Repeat([]byte("deterministic compressor payload"), 256)
	opts := &CompressOptions{Checksum: ChecksumSigned, SearchLimit: WindowSize, MinMatchLength: MinMatch2}

	first, err := Compress(input, opts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compress(input, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("compressed output differs between identical calls")
	}
}

func TestCompressToWriterMatchesCompress(t *testing.T) {
	for _, corpus := range benchmarkCorpora {
		for _, minMatch := range []int{MinMatch2, MinMatchDefault} {
			for _, searchLimit := range []int{0, 64, 2048, WindowSize} {
				opts := &CompressOptions{
					Checksum:       ChecksumSigned,
					SearchLimit:    searchLimit,
					MinMatchLength: minMatch,
				}
				want, err := Compress(corpus.data, opts)
				if err != nil {
					t.Fatal(err)
				}

				var got bytes.Buffer
				_, _, err = CompressToWriter(&got, bytes.NewReader(corpus.data), opts)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got.Bytes(), want) {
					t.Fatalf("corpus=%s minMatch=%d searchLimit=%d: stream output differs", corpus.name, minMatch, searchLimit)
				}
			}
		}
	}
}

func TestCompressSearchLimitBoundary(t *testing.T) {
	block := make([]byte, 3072)
	_, _ = rand.New(rand.NewSource(2)).Read(block)
	input := append(append([]byte(nil), block...), block...)

	inside, err := Compress(input, &CompressOptions{SearchLimit: 4096})
	if err != nil {
		t.Fatal(err)
	}
	outside, err := Compress(input, &CompressOptions{SearchLimit: 2048})
	if err != nil {
		t.Fatal(err)
	}
	if len(inside) >= len(outside) {
		t.Fatalf("larger search limit did not find long-distance matches: inside=%d outside=%d", len(inside), len(outside))
	}
}

func TestCompressAvoidsOverlapBackrefs(t *testing.T) {
	input := bytes.Repeat([]byte("A"), 4096)
	encoded, err := Compress(input, DefaultCompressOptions())
	if err != nil {
		t.Fatal(err)
	}

	inPos := 0
	produced := 0
	for produced < len(input) {
		flags := encoded[inPos]
		inPos++
		for bit := range FlagBits {
			if produced >= len(input) {
				break
			}
			if (flags>>bit)&1 == 1 {
				inPos++
				produced++
				continue
			}

			lo := int(encoded[inPos])
			hi := int(encoded[inPos+1])
			inPos += 2
			offset := lo + ((hi & 0xf0) << 4)
			length := (hi & 0x0f) + MinMatchDefault
			if offset < length {
				t.Fatalf("overlap backref generated: offset=%d length=%d", offset, length)
			}
			produced += length
		}
	}
}

func TestMatchFinderInputLimit(t *testing.T) {
	if matchFinderInputTooLarge(math.MaxInt32) {
		t.Fatal("maximum supported input rejected")
	}
	if strconv.IntSize == 64 && !matchFinderInputTooLarge(int(int64(math.MaxInt32)+1)) {
		t.Fatal("oversized input accepted")
	}
}

func TestDecompressBlockLenientModesIgnoreChecksum(t *testing.T) {
	raw := bytes.Repeat([]byte{0x00, 0x7f, 0x80, 0xff}, 128)

	for _, checksum := range []ChecksumMode{ChecksumUnsigned, ChecksumSigned} {
		encoded, err := Compress(raw, &CompressOptions{Checksum: checksum, SearchLimit: 0})
		if err != nil {
			t.Fatal(err)
		}
		encoded[len(encoded)-1] ^= 0xff

		opts := &Options{Checksum: checksum, VerifyChecksum: false}
		decoded, consumed, err := DecompressBlock(encoded, len(raw), opts)
		if err != nil {
			t.Fatal(err)
		}
		if consumed != len(encoded) || !bytes.Equal(decoded, raw) {
			t.Fatalf("lenient decode mismatch: consumed=%d encoded=%d", consumed, len(encoded))
		}
	}
}

func TestDecompressBlockTruncatedLiteralRun(t *testing.T) {
	_, consumed, err := DecompressBlock([]byte{0xff, 1, 2, 3, 4}, 8, DefaultOptions())
	if !errors.Is(err, ErrUnexpectedEOFBit) {
		t.Fatalf("want ErrUnexpectedEOFBit, got %v", err)
	}
	if consumed != 1 {
		t.Fatalf("consumed=%d want=1", consumed)
	}
}

func TestDecompressToWriterPropagatesWriterError(t *testing.T) {
	wantErr := errors.New("write failed")
	raw := bytes.Repeat([]byte("writer failure payload"), 4096)
	encoded, err := Compress(raw, nil)
	if err != nil {
		t.Fatal(err)
	}

	consumed, err := DecompressToWriter(errorWriter{err: wantErr}, bytes.NewReader(encoded), len(raw), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want writer error, got %v", err)
	}
	if consumed <= 0 || consumed >= int64(len(encoded)) {
		t.Fatalf("unexpected consumed bytes after writer failure: %d", consumed)
	}
}

// errorWriter returns a configured error for every write.
type errorWriter struct {
	err error
}

func (writer errorWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

// appendChecksum appends the checksum for output to an encoded payload.
func appendChecksum(payload, output []byte, mode ChecksumMode) []byte {
	encoded := append([]byte(nil), payload...)
	var checksum int32
	if mode == ChecksumSigned {
		checksum = sumSigned(output)
	} else {
		checksum = sumUnsigned(output)
	}

	var checksumBytes [4]byte
	binary.LittleEndian.PutUint32(checksumBytes[:], uint32(checksum))
	return append(encoded, checksumBytes[:]...)
}
