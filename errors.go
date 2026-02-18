// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import "errors"

// Package errors. Use errors.New for static messages, fmt.Errorf when values are needed.
var (
	// ErrInputTooShort indicates that there are not enough bytes to read the trailing checksum.
	ErrInputTooShort = errors.New("not enough data for checksum")
	// ErrUnexpectedEOF indicates that input ended while reading a new flags byte.
	ErrUnexpectedEOF = errors.New("unexpected end of input while reading flags")
	// ErrUnexpectedEOFBit indicates that input ended in the middle of an 8-slot flags group.
	ErrUnexpectedEOFBit = errors.New("unexpected end of input inside flags block")
	// ErrTrailingData indicates that bytes remain after one full LZSS block is decoded.
	ErrTrailingData = errors.New("trailing bytes after lzss block")
	// ErrNilReader indicates that a required io.Reader argument was nil.
	ErrNilReader = errors.New("reader is nil")
	// ErrNilWriter indicates that a required io.Writer argument was nil.
	ErrNilWriter = errors.New("writer is nil")
	// ErrNilOutLenProvider indicates that the callback for providing output length was nil.
	ErrNilOutLenProvider = errors.New("outLen provider is nil")
	// ErrNegativeOutLen indicates that a requested output length is negative.
	ErrNegativeOutLen = errors.New("output length must be non-negative")
	// ErrEmptyInput indicates that the provided compressed input is empty.
	ErrEmptyInput = errors.New("input is empty")
)
