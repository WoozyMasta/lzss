package lzss

import (
	"bytes"
	"errors"
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

func TestRoundTripMinMatch2(t *testing.T) {
	// Data that yields 2-byte back-refs: "abab..." so at position 2 we match "ab" at 0 (min match 2).
	input := bytes.Repeat([]byte("ab"), 64)
	copts := &CompressOptions{
		Checksum:       ChecksumUnsigned,
		SearchLimit:    128,
		MinMatchLength: MinMatch2,
	}
	enc, err := Compress(input, copts)
	if err != nil {
		t.Fatal(err)
	}
	dopts := &Options{Checksum: ChecksumUnsigned, VerifyChecksum: true, MinMatchLength: MinMatch2}
	dec, err := Decompress(enc, len(input), dopts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, dec) {
		t.Fatalf("min match 2 round-trip: got %d bytes, want %d", len(dec), len(input))
	}
	// Encoded with min match 2 should be smaller than all-literals (back-refs used).
	if len(enc) >= len(input)+4 {
		t.Fatalf("expected compression to use back-refs (len(enc)=%d)", len(enc))
	}
}

func TestDecompressMinMatch2Option(t *testing.T) {
	// Compress with min match 2, decompress with explicit MinMatchLength 2 (and zero => default 3).
	raw := []byte("xyxyxyxy")
	copts := &CompressOptions{SearchLimit: 8, MinMatchLength: MinMatch2, Checksum: ChecksumUnsigned}
	enc, err := Compress(raw, copts)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decompress(enc, len(raw), &Options{MinMatchLength: MinMatch2, VerifyChecksum: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, dec) {
		t.Fatalf("got %q", dec)
	}
}

func TestDecompressBlockConsumesFirstBlockOnly(t *testing.T) {
	rawA := []byte("first block data")
	rawB := []byte("second block payload")
	encA, err := Compress(rawA, nil)
	if err != nil {
		t.Fatal(err)
	}
	encB, err := Compress(rawB, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := append(append([]byte{}, encA...), encB...)

	decA, consumedA, err := DecompressBlock(joined, len(rawA), nil)
	if err != nil {
		t.Fatal(err)
	}
	if consumedA != len(encA) {
		t.Fatalf("consumedA=%d want=%d", consumedA, len(encA))
	}
	if !bytes.Equal(decA, rawA) {
		t.Fatalf("got %q", decA)
	}

	decB, consumedB, err := DecompressBlock(joined[consumedA:], len(rawB), nil)
	if err != nil {
		t.Fatal(err)
	}
	if consumedB != len(encB) {
		t.Fatalf("consumedB=%d want=%d", consumedB, len(encB))
	}
	if !bytes.Equal(decB, rawB) {
		t.Fatalf("got %q", decB)
	}
}

func TestDecompressFromReaderStopsAtBlockBoundary(t *testing.T) {
	rawA := []byte("reader block alpha")
	rawB := []byte("reader block beta")
	encA, err := Compress(rawA, nil)
	if err != nil {
		t.Fatal(err)
	}
	encB, err := Compress(rawB, nil)
	if err != nil {
		t.Fatal(err)
	}

	stream := bytes.NewReader(append(append([]byte{}, encA...), encB...))
	decA, consumedA, err := DecompressFromReader(stream, len(rawA), nil)
	if err != nil {
		t.Fatal(err)
	}
	if consumedA != int64(len(encA)) {
		t.Fatalf("consumedA=%d want=%d", consumedA, len(encA))
	}
	if !bytes.Equal(decA, rawA) {
		t.Fatalf("got %q", decA)
	}

	pos, err := stream.Seek(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if pos != consumedA {
		t.Fatalf("reader pos=%d want=%d", pos, consumedA)
	}

	decB, consumedB, err := DecompressFromReader(stream, len(rawB), nil)
	if err != nil {
		t.Fatal(err)
	}
	if consumedB != int64(len(encB)) {
		t.Fatalf("consumedB=%d want=%d", consumedB, len(encB))
	}
	if !bytes.Equal(decB, rawB) {
		t.Fatalf("got %q", decB)
	}
}

func TestDecompressRejectsTrailingData(t *testing.T) {
	raw := []byte("payload")
	enc, err := Compress(raw, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Decompress(append(append([]byte{}, enc...), 0xAA), len(raw), nil)
	if !errors.Is(err, ErrTrailingData) {
		t.Fatalf("want ErrTrailingData, got %v", err)
	}
}

func TestDecompressNFromReader(t *testing.T) {
	rawA := []byte("multi block A")
	rawB := []byte("multi block B")
	rawC := []byte("multi block C")
	encA, err := Compress(rawA, nil)
	if err != nil {
		t.Fatal(err)
	}
	encB, err := Compress(rawB, nil)
	if err != nil {
		t.Fatal(err)
	}
	encC, err := Compress(rawC, nil)
	if err != nil {
		t.Fatal(err)
	}

	stream := bytes.NewReader(append(append(append([]byte{}, encA...), encB...), encC...))
	blocks, consumed, err := DecompressNFromReader(stream, []int{len(rawA), len(rawB), len(rawC)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if consumed != int64(len(encA)+len(encB)+len(encC)) {
		t.Fatalf("consumed=%d", consumed)
	}
	if len(blocks) != 3 {
		t.Fatalf("blocks=%d", len(blocks))
	}
	if !bytes.Equal(blocks[0], rawA) || !bytes.Equal(blocks[1], rawB) || !bytes.Equal(blocks[2], rawC) {
		t.Fatalf("decoded blocks mismatch")
	}
}

func TestDecompressUntilEOF(t *testing.T) {
	rawA := []byte("until eof one")
	rawB := []byte("until eof two")
	encA, err := Compress(rawA, nil)
	if err != nil {
		t.Fatal(err)
	}
	encB, err := Compress(rawB, nil)
	if err != nil {
		t.Fatal(err)
	}

	stream := bytes.NewReader(append(append([]byte{}, encA...), encB...))
	lengths := []int{len(rawA), len(rawB)}
	index := 0
	next := func() (int, bool) {
		if index >= len(lengths) {
			return 0, false
		}
		outLen := lengths[index]
		index++

		return outLen, true
	}

	blocks, consumed, err := DecompressUntilEOF(stream, next, nil)
	if err != nil {
		t.Fatal(err)
	}
	if consumed != int64(len(encA)+len(encB)) {
		t.Fatalf("consumed=%d", consumed)
	}
	if len(blocks) != 2 {
		t.Fatalf("blocks=%d", len(blocks))
	}
	if !bytes.Equal(blocks[0], rawA) || !bytes.Equal(blocks[1], rawB) {
		t.Fatalf("decoded blocks mismatch")
	}
}

func TestDecompressUntilEOFNilProvider(t *testing.T) {
	_, _, err := DecompressUntilEOF(bytes.NewReader(nil), nil, nil)
	if !errors.Is(err, ErrNilOutLenProvider) {
		t.Fatalf("want ErrNilOutLenProvider, got %v", err)
	}
}
