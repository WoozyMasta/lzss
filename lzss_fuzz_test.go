// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import (
	"bytes"
	"testing"
)

func FuzzRoundTripAllPaths(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("a"),
		bytes.Repeat([]byte("a"), 128),
		bytes.Repeat([]byte("ab"), 128),
		[]byte("literal-heavy seed with no intentional repetition"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) == 0 || len(input) > 8*1024 {
			t.Skip()
		}

		for _, minMatch := range []int{MinMatchDefault, MinMatch2} {
			compressOpts := &CompressOptions{
				Checksum:       ChecksumUnsigned,
				SearchLimit:    256,
				MinMatchLength: minMatch,
			}
			decompressOpts := &Options{
				Checksum:       ChecksumUnsigned,
				VerifyChecksum: true,
				MinMatchLength: minMatch,
			}

			encoded, err := Compress(input, compressOpts)
			if err != nil {
				t.Fatal(err)
			}
			assertDecodePathsEqual(t, input, encoded, decompressOpts)

			var streamEncoded bytes.Buffer
			_, _, err = CompressToWriter(&streamEncoded, bytes.NewReader(input), compressOpts)
			if err != nil {
				t.Fatal(err)
			}
			assertDecodePathsEqual(t, input, streamEncoded.Bytes(), decompressOpts)
		}
	})
}

func FuzzDecodePathParity(f *testing.F) {
	f.Add([]byte{1, 'x', 'x', 0, 0, 0}, 1, false, true)
	f.Add([]byte{0, 1, 15, 0x13, 0x04, 0, 0}, 18, true, true)
	f.Add([]byte{0xff}, 8, false, false)

	f.Fuzz(func(t *testing.T, encoded []byte, outLen int, signed, verify bool) {
		if outLen < 0 || outLen > 8*1024 || len(encoded) > 16*1024 {
			t.Skip()
		}

		opts := &Options{VerifyChecksum: verify}
		if signed {
			opts.Checksum = ChecksumSigned
		}

		sliceOut, sliceConsumed, sliceErr := DecompressBlock(encoded, outLen, opts)
		readerOut, readerConsumed, readerErr := DecompressFromReader(bytes.NewReader(encoded), outLen, opts)
		if (sliceErr == nil) != (readerErr == nil) {
			t.Fatalf("decode result differs: slice=%v reader=%v", sliceErr, readerErr)
		}
		if sliceErr != nil {
			return
		}
		if sliceConsumed != int(readerConsumed) || !bytes.Equal(sliceOut, readerOut) {
			t.Fatalf("successful decode differs: slice=%d reader=%d", sliceConsumed, readerConsumed)
		}

		var writerOut bytes.Buffer
		writerConsumed, writerErr := DecompressToWriter(&writerOut, bytes.NewReader(encoded), outLen, opts)
		if writerErr != nil {
			t.Fatalf("writer decode failed after successful slice decode: %v", writerErr)
		}
		if sliceConsumed != int(writerConsumed) || !bytes.Equal(sliceOut, writerOut.Bytes()) {
			t.Fatalf("writer decode differs: slice=%d writer=%d", sliceConsumed, writerConsumed)
		}
	})
}

func assertDecodePathsEqual(t *testing.T, want, encoded []byte, opts *Options) {
	t.Helper()

	got, consumed, err := DecompressBlock(encoded, len(want), opts)
	if err != nil {
		t.Fatal(err)
	}
	if consumed != len(encoded) || !bytes.Equal(got, want) {
		t.Fatalf("slice decode mismatch: consumed=%d encoded=%d", consumed, len(encoded))
	}

	got, consumed64, err := DecompressFromReader(bytes.NewReader(encoded), len(want), opts)
	if err != nil {
		t.Fatal(err)
	}
	if consumed64 != int64(len(encoded)) || !bytes.Equal(got, want) {
		t.Fatalf("reader decode mismatch: consumed=%d encoded=%d", consumed64, len(encoded))
	}

	var streamOut bytes.Buffer
	consumed64, err = DecompressToWriter(&streamOut, bytes.NewReader(encoded), len(want), opts)
	if err != nil {
		t.Fatal(err)
	}
	if consumed64 != int64(len(encoded)) || !bytes.Equal(streamOut.Bytes(), want) {
		t.Fatalf("writer decode mismatch: consumed=%d encoded=%d", consumed64, len(encoded))
	}
}
