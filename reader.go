// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import "io"

// countingByteReader reads from a byte reader and counts the number of bytes read.
type countingByteReader struct {
	base       io.ByteReader // The byte reader to read from.
	baseReader io.Reader     // The reader used for exact span reads.
	count      int64         // The number of bytes read.
	scratch    [MaxMatch]byte
}

// Read reads p from the reader and increments the count.
func (r *countingByteReader) Read(p []byte) (int, error) {
	n, err := r.baseReader.Read(p)
	r.count += int64(n)
	return n, err
}

// readFull reads exactly len(p) bytes without reading past p.
func (r *countingByteReader) readFull(p []byte) error {
	for len(p) > 0 {
		n, err := r.Read(p)
		p = p[n:]
		if err != nil {
			if err == io.EOF && len(p) > 0 {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}

// ReadByte reads a byte from the reader and increments the count.
func (r *countingByteReader) ReadByte() (byte, error) {
	b, err := r.base.ReadByte()
	if err != nil {
		return 0, err
	}

	r.count++

	return b, nil
}
