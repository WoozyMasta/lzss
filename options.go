package lzss

// ChecksumMode defines how the 4-byte checksum is computed.
type ChecksumMode int

// Checksum mode constants.
const (
	ChecksumUnsigned ChecksumMode = iota // Sum bytes as uint8 (default for archives).
	ChecksumSigned                       // Sum bytes as int8 (used by some texture formats).
)

// Options configures Decompress and Compress behavior.
type Options struct {
	// Checksum sets unsigned vs signed checksum.
	Checksum ChecksumMode
	// VerifyChecksum: if true, Decompress returns an error on checksum mismatch.
	// If false, mismatch is ignored (lenient mode for formats with often-bad checksums).
	VerifyChecksum bool
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
