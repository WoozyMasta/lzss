// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

import "errors"

// Package errors. Use errors.New for static messages, fmt.Errorf when values are needed.
var (
	ErrInputTooShort     = errors.New("not enough data for checksum")
	ErrUnexpectedEOF     = errors.New("unexpected end of input while reading flags")
	ErrUnexpectedEOFBit  = errors.New("unexpected end of input inside flags block")
	ErrTrailingData      = errors.New("trailing bytes after lzss block")
	ErrNilReader         = errors.New("reader is nil")
	ErrNilOutLenProvider = errors.New("outLen provider is nil")
	ErrNegativeOutLen    = errors.New("output length must be non-negative")
	ErrEmptyInput        = errors.New("input is empty")
)
