// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import (
	"bytes"
	"encoding/binary"
	"errors"
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

type errorWriter struct {
	err error
}

func (writer errorWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

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
