package aggregator

const (
	errWindowSize = 60
)

// errWindow is a circular buffer that counts errors per second over the last 60 s.
// bucket[i] holds the error count for the Unix second (i % errWindowSize).
// epoch[i] records which Unix second that bucket belongs to; stale buckets are zeroed.
type errWindow struct {
	buckets [errWindowSize]int32
	epochs  [errWindowSize]int64 // Unix seconds
}

// record adds one error at the given Unix second.
func (w *errWindow) record(sec int64) {
	idx := int(sec % errWindowSize)

	if w.epochs[idx] != sec {
		// New second: reset the stale bucket.
		w.buckets[idx] = 0
		w.epochs[idx] = sec
	}

	w.buckets[idx]++
}

// sum returns the total errors in the last 60 s relative to now (Unix second).
func (w *errWindow) sum(now int64) int64 {
	var total int64

	for i := range w.epochs {
		if now-w.epochs[i] < errWindowSize {
			total += int64(w.buckets[i])
		}
	}

	return total
}
