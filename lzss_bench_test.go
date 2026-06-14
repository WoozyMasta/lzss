// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/rand"
	"strconv"
	"testing"
)

const benchmarkCorpusSize = 64 * 1024

type benchmarkCorpus struct {
	name string
	data []byte
}

var benchmarkCorpora = newBenchmarkCorpora()

func newBenchmarkCorpora() []benchmarkCorpus {
	incompressible := make([]byte, benchmarkCorpusSize)
	_, _ = rand.New(rand.NewSource(1)).Read(incompressible)

	longDistance := make([]byte, 0, benchmarkCorpusSize)
	for len(longDistance) < benchmarkCorpusSize {
		block := make([]byte, 3072)
		_, _ = rand.New(rand.NewSource(int64(len(longDistance) + 1))).Read(block)
		longDistance = append(longDistance, block...)
		longDistance = append(longDistance, block...)
	}

	return []benchmarkCorpus{
		{name: "text", data: bytes.Repeat([]byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. "), 1150)[:benchmarkCorpusSize]},
		{name: "repetitive", data: bytes.Repeat([]byte("A"), benchmarkCorpusSize)},
		{name: "incompressible", data: incompressible},
		{name: "long-distance", data: longDistance[:benchmarkCorpusSize]},
	}
}

func BenchmarkCompressCorpus(b *testing.B) {
	for _, corpus := range benchmarkCorpora {
		b.Run(corpus.name, func(b *testing.B) {
			opts := DefaultCompressOptions()
			encoded, err := Compress(corpus.data, opts)
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(corpus.data)))
			b.ResetTimer()
			for b.Loop() {
				_, _ = Compress(corpus.data, opts)
			}
			b.ReportMetric(compressionRatio(encoded, corpus.data), "ratio")
		})
	}
}

func BenchmarkCompressToWriterCorpus(b *testing.B) {
	for _, corpus := range benchmarkCorpora {
		b.Run(corpus.name, func(b *testing.B) {
			opts := DefaultCompressOptions()
			var encoded bytes.Buffer
			_, _, err := CompressToWriter(&encoded, bytes.NewReader(corpus.data), opts)
			if err != nil {
				b.Fatal(err)
			}

			reader := bytes.NewReader(corpus.data)
			b.ReportAllocs()
			b.SetBytes(int64(len(corpus.data)))
			b.ResetTimer()
			for b.Loop() {
				reader.Reset(corpus.data)
				_, _, _ = CompressToWriter(io.Discard, reader, opts)
			}
			b.ReportMetric(compressionRatio(encoded.Bytes(), corpus.data), "ratio")
		})
	}
}

func BenchmarkCompressSearchLimit(b *testing.B) {
	for _, corpus := range benchmarkCorpora {
		if corpus.name != "text" && corpus.name != "incompressible" {
			continue
		}

		for _, limit := range []int{0, 64, 256, 1024, 2048, 4096} {
			opts := &CompressOptions{Checksum: ChecksumUnsigned, SearchLimit: limit}
			encoded, err := Compress(corpus.data, opts)
			if err != nil {
				b.Fatal(err)
			}

			b.Run(corpus.name+"/limit="+strconv.Itoa(limit), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(corpus.data)))
				b.ResetTimer()
				for b.Loop() {
					_, _ = Compress(corpus.data, opts)
				}
				b.ReportMetric(compressionRatio(encoded, corpus.data), "ratio")
			})
		}
	}
}

func BenchmarkDecompressCorpus(b *testing.B) {
	for _, corpus := range benchmarkCorpora {
		for _, verify := range []bool{true, false} {
			opts := &Options{Checksum: ChecksumUnsigned, VerifyChecksum: verify}
			encoded, err := Compress(corpus.data, benchmarkEncodeOptions(corpus.name))
			if err != nil {
				b.Fatal(err)
			}

			b.Run(corpus.name+"/verify="+strconv.FormatBool(verify), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(corpus.data)))
				b.ResetTimer()
				for b.Loop() {
					_, _, _ = DecompressBlock(encoded, len(corpus.data), opts)
				}
			})
		}
	}
}

func BenchmarkDecompressFromReaderCorpus(b *testing.B) {
	benchmarkReaderDecode(b, func(encoded []byte, outLen int, opts *Options) {
		reader := bytes.NewReader(encoded)
		_, _, _ = DecompressFromReader(reader, outLen, opts)
	})
}

func BenchmarkDecompressToWriterCorpus(b *testing.B) {
	benchmarkReaderDecode(b, func(encoded []byte, outLen int, opts *Options) {
		reader := bytes.NewReader(encoded)
		_, _ = DecompressToWriter(io.Discard, reader, outLen, opts)
	})
}

func benchmarkReaderDecode(b *testing.B, decode func([]byte, int, *Options)) {
	b.Helper()

	for _, corpus := range benchmarkCorpora {
		for _, verify := range []bool{true, false} {
			opts := &Options{Checksum: ChecksumUnsigned, VerifyChecksum: verify}
			encoded, err := Compress(corpus.data, benchmarkEncodeOptions(corpus.name))
			if err != nil {
				b.Fatal(err)
			}

			b.Run(corpus.name+"/verify="+strconv.FormatBool(verify), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(corpus.data)))
				b.ResetTimer()
				for b.Loop() {
					decode(encoded, len(corpus.data), opts)
				}
			})
		}
	}
}

func BenchmarkChecksum(b *testing.B) {
	data := benchmarkCorpora[2].data
	b.SetBytes(int64(len(data)))

	b.Run("unsigned", func(b *testing.B) {
		var sum int32
		for b.Loop() {
			sum = sumUnsigned(data)
		}
		_ = sum
	})
	b.Run("signed", func(b *testing.B) {
		var sum int32
		for b.Loop() {
			sum = sumSigned(data)
		}
		_ = sum
	})
}

func BenchmarkDecompressChecksumMode(b *testing.B) {
	data := benchmarkCorpora[2].data

	for _, checksum := range []ChecksumMode{ChecksumUnsigned, ChecksumSigned} {
		name := "unsigned"
		if checksum == ChecksumSigned {
			name = "signed"
		}

		encoded, err := Compress(data, &CompressOptions{Checksum: checksum, SearchLimit: 0})
		if err != nil {
			b.Fatal(err)
		}
		opts := &Options{Checksum: checksum, VerifyChecksum: true}

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				_, _, _ = DecompressBlock(encoded, len(data), opts)
			}
		})
	}
}

func BenchmarkDecompressOverlapBackref(b *testing.B) {
	encoded := makeOverlapBackrefBlock(benchmarkCorpusSize)
	opts := DefaultOptions()

	b.Run("slice", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(benchmarkCorpusSize)
		for b.Loop() {
			_, _, _ = DecompressBlock(encoded, benchmarkCorpusSize, opts)
		}
	})
	b.Run("writer", func(b *testing.B) {
		reader := bytes.NewReader(encoded)
		b.ReportAllocs()
		b.SetBytes(benchmarkCorpusSize)
		for b.Loop() {
			reader.Reset(encoded)
			_, _ = DecompressToWriter(io.Discard, reader, benchmarkCorpusSize, opts)
		}
	})
}

func benchmarkEncodeOptions(corpus string) *CompressOptions {
	opts := DefaultCompressOptions()
	if corpus == "incompressible" {
		opts.SearchLimit = 0
	}
	return opts
}

func compressionRatio(encoded, raw []byte) float64 {
	return float64(len(encoded)) / float64(len(raw))
}

func makeOverlapBackrefBlock(outLen int) []byte {
	const (
		literal = byte('A')
		length  = MaxMatch
	)

	out := make([]byte, 0, outLen/length*2)
	produced := 0
	for produced < outLen {
		flagPos := len(out)
		out = append(out, 0)
		var flags byte

		for bit := 0; bit < FlagBits && produced < outLen; bit++ {
			if produced == 0 {
				flags |= 1 << bit
				out = append(out, literal)
				produced++
				continue
			}

			out = append(out, 1, length-MinMatchDefault)
			produced += length
		}
		out[flagPos] = flags
	}

	var checksum [4]byte
	binary.LittleEndian.PutUint32(checksum[:], uint32(literal)*uint32(outLen))
	return append(out, checksum[:]...)
}
