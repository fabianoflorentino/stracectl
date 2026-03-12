package tracer

// unameStr converts an uname field (slice of int8 or byte) to a Go string,
// stopping at the first NUL byte. It accepts element types int8 or uint8.
func unameStr[B ~int8 | ~uint8](b []B) string {
	buf := make([]byte, 0, len(b))
	for _, v := range b {
		if v == 0 {
			break
		}
		buf = append(buf, byte(v))
	}
	return string(buf)
}
