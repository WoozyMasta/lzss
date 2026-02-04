package lzss

import (
	"bytes"
	"testing"
)

func TestDecompressNilOptions(t *testing.T) {
	// Nil opts => default (unsigned, strict)
	raw := []byte("hello world")
	enc, err := Compress(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decompress(enc, len(raw), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, dec) {
		t.Fatalf("got %q", dec)
	}
}

func TestRoundTripDefaultOptions(t *testing.T) {
	// Use 256 bytes so we get 32 groups (all literals) and avoid partial-group edge cases
	input := bytes.Repeat([]byte("abcdefgh"), 32)
	opts := DefaultCompressOptions()
	opts.SearchLimit = 1024
	enc, err := Compress(input, opts)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decompress(enc, len(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, dec) {
		t.Fatalf("lengths: in=%d dec=%d", len(input), len(dec))
	}
}

func TestRoundTripSignedLenient(t *testing.T) {
	input := []byte("signed lenient round trip data here")
	copts := &CompressOptions{Checksum: ChecksumSigned, SearchLimit: 512}
	enc, err := Compress(input, copts)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decompress(enc, len(input), SignedLenientOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, dec) {
		t.Fatalf("got %q", dec)
	}
}

func TestOverlappingBackReference(t *testing.T) {
	// Data that compresses to back-references; decoder must handle overlap (offset < need) with byte-by-byte copy
	input := bytes.Repeat([]byte("a"), 128)
	enc, err := Compress(input, &CompressOptions{SearchLimit: 4096, Checksum: ChecksumUnsigned})
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decompress(enc, len(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, dec) {
		t.Fatalf("overlap: got %d bytes, want %d; first 16 = %x", len(dec), len(input), dec[:min(16, len(dec))])
	}
}

func TestEmptyInputCompress(t *testing.T) {
	_, err := Compress(nil, nil)
	if err != ErrEmptyInput {
		t.Fatalf("want ErrEmptyInput, got %v", err)
	}
}

func TestInputTooShortDecompress(t *testing.T) {
	_, err := Decompress([]byte{1, 2}, 10, nil)
	if err != ErrInputTooShort {
		t.Fatalf("want ErrInputTooShort, got %v", err)
	}
}

func TestChecksumStrictMismatch(t *testing.T) {
	raw := []byte("x")
	enc, err := Compress(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	enc[len(enc)-1] ^= 0xFF
	_, err = Decompress(enc, len(raw), DefaultOptions())
	if err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestChecksumLenientNoError(t *testing.T) {
	raw := []byte("y")
	enc, err := Compress(raw, &CompressOptions{Checksum: ChecksumSigned, SearchLimit: 0})
	if err != nil {
		t.Fatal(err)
	}
	enc[len(enc)-1] ^= 0xFF
	dec, err := Decompress(enc, len(raw), SignedLenientOptions())
	if err != nil {
		t.Fatalf("lenient should not error: %v", err)
	}
	if !bytes.Equal(raw, dec) {
		t.Fatalf("got %q", dec)
	}
}

func TestLiteralsOnlyCompress(t *testing.T) {
	input := []byte("no matches here")
	enc, err := Compress(input, &CompressOptions{SearchLimit: 0, Checksum: ChecksumUnsigned})
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decompress(enc, len(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, dec) {
		t.Fatalf("got %q", dec)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
