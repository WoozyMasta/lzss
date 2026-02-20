package lzss

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

var benchInput = bytes.Repeat([]byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. "), 512)

func BenchmarkCompress(b *testing.B) {
	data := benchInput
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Compress(data, DefaultCompressOptions())
	}
}

func BenchmarkCompressToWriter(b *testing.B) {
	data := benchInput

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := bytes.NewReader(data)
		_, _, _ = CompressToWriter(io.Discard, src, DefaultCompressOptions())
	}
}

func BenchmarkCompressSearchLevels(b *testing.B) {
	data := benchInput
	levels := []int{0, 64, 256, 1024, 2048, 4096}
	for _, limit := range levels {
		opts := &CompressOptions{Checksum: ChecksumUnsigned, SearchLimit: limit}
		b.Run(fmt.Sprintf("Limit=%d", limit), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = Compress(data, opts)
			}
		})
	}
}

func BenchmarkDecompress(b *testing.B) {
	data := benchInput
	enc, err := Compress(data, DefaultCompressOptions())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decompress(enc, len(data), nil)
	}
}

func BenchmarkDecompressFromReader(b *testing.B) {
	data := benchInput
	enc, err := Compress(data, DefaultCompressOptions())
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(enc)
		_, _, _ = DecompressFromReader(r, len(data), nil)
	}
}

func BenchmarkDecompressToWriterDiscard(b *testing.B) {
	data := benchInput
	enc, err := Compress(data, DefaultCompressOptions())
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(enc)
		_, _ = DecompressToWriter(io.Discard, r, len(data), nil)
	}
}

func BenchmarkDecompressToWriterBuffer(b *testing.B) {
	data := benchInput
	enc, err := Compress(data, DefaultCompressOptions())
	if err != nil {
		b.Fatal(err)
	}

	var out bytes.Buffer
	out.Grow(len(data))

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out.Reset()
		r := bytes.NewReader(enc)
		_, _ = DecompressToWriter(&out, r, len(data), nil)
	}
}
