package lzss

import "io"

// sliceByteReader reads from a byte slice.
type sliceByteReader struct {
	data []byte // The byte slice to read from.
	pos  int    // The current position in the byte slice.
}

// countingByteReader reads from a byte reader and counts the number of bytes read.
type countingByteReader struct {
	base  io.ByteReader // The byte reader to read from.
	count int64         // The number of bytes read.
}

// ReadByte reads a byte from the slice.
func (r *sliceByteReader) ReadByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	b := r.data[r.pos]
	r.pos++

	return b, nil
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
