// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Maxim Levchenko (WoozyMasta)
// Source: github.com/woozymasta/lzss

package lzss

// LZSS:8bit format constants.
const (
	// WindowSize is the sliding window size (ring buffer).
	WindowSize = 4096

	// MaxMatch is the maximum back-reference length when MinMatchLength is 3 (encoded 3..18).
	MaxMatch = 18

	// Filler is the fill byte when back-reference offset is before start of output.
	Filler = 0x20

	// FlagBits is the number of bits per flag byte (one flag byte per 8 slots: literal or pointer).
	FlagBits = 8

	// MinMatchDefault is the default minimum back-reference length (3..18). Use MinMatch2 for range 2..17.
	MinMatchDefault = 3

	// MinMatch2 is the minimum back-reference length when nibble encodes length-2, range 2..17.
	MinMatch2 = 2
)
