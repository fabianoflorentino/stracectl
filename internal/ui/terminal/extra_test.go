package terminal

import "testing"

func Test_RecordEvents_NoPanic(t *testing.T) {
	// Ensure these functions do not panic and write to /tmp.
	RecordFallbackEvent(80, 24)
	RecordUIEvent("resize", 80, 24)
}
