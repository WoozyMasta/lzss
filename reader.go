// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import "io"

// countingByteReader reads from a byte reader and counts the number of bytes read.
type countingByteReader struct {
	base  io.ByteReader // The byte reader to read from.
	count int64         // The number of bytes read.
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
