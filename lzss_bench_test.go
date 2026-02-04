package lzss

import (
	"bytes"
	"fmt"
	"testing"
)

var benchInput = bytes.Repeat([]byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. "), 512)

func BenchmarkCompress(b *testing.B) {
	data := benchInput
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Compress(data, DefaultCompressOptions())
	}
}

func BenchmarkCompressSearchLevels(b *testing.B) {
	data := benchInput
	levels := []int{0, 64, 256, 1024, 2048, 4096}
	for _, limit := range levels {
		limit := limit
		opts := &CompressOptions{Checksum: ChecksumUnsigned, SearchLimit: limit}
		b.Run(fmt.Sprintf("Limit=%d", limit), func(b *testing.B) {
			b.ReportAllocs()
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decompress(enc, len(data), nil)
	}
}
