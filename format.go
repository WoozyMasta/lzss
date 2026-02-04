package lzss

// LZSS:8bit format constants.
const (
	WindowSize = 4096 // Sliding window size (ring buffer).
	MaxMatch   = 18   // Maximum back-reference length (encoded as 3..18).
	Filler     = 0x20 // Fill byte when back-reference offset is before start of output.
	FlagBits   = 8    // Bits per flag byte (one flag byte per 8 slots: literal or pointer).
)
