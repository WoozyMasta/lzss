package lzss

// sumUnsigned returns the unsigned sum of bytes (for comparison with stored uint32).
func sumUnsigned(data []byte) int32 {
	var s int32
	for _, b := range data {
		s += int32(b)
	}

	return s
}

// sumSigned returns the signed sum of bytes (for comparison with stored int32).
func sumSigned(data []byte) int32 {
	var s int32
	for _, b := range data {
		s += int32(int8(b))
	}

	return s
}
