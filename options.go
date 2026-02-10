package lzss

// ChecksumMode defines how the 4-byte checksum is computed.
type ChecksumMode int

// Checksum mode constants.
const (
	// Sum bytes as uint8 (default for archives).
	ChecksumUnsigned ChecksumMode = iota

	// Sum bytes as int8 (used by some texture formats).
	ChecksumSigned
)

// Options configures Decompress and Compress behavior.
type Options struct {
	// Checksum sets unsigned vs signed checksum.
	Checksum ChecksumMode
	// VerifyChecksum: if true, Decompress returns an error on checksum mismatch.
	// If false, mismatch is ignored (lenient mode for formats with often-bad checksums).
	VerifyChecksum bool
	// MinMatchLength is the minimum back-reference length used when decoding the length nibble.
	//  - 3 (default): nibble + 3 -> length 3..18.
	//  - 2: nibble + 2 -> length 2..17.
	// Zero is treated as 3.
	MinMatchLength int
}

// DefaultOptions returns options for default behavior: unsigned checksum, strict verification.
func DefaultOptions() *Options {
	return &Options{
		Checksum:       ChecksumUnsigned,
		VerifyChecksum: true,
	}
}

// SignedLenientOptions returns options: signed checksum, do not return error on mismatch.
func SignedLenientOptions() *Options {
	return &Options{
		Checksum:       ChecksumSigned,
		VerifyChecksum: false,
	}
}
