package aggregator

import (
	"math/bits"
	"time"
)

// latencyBuckets is the number of log2 histogram buckets used for percentile estimation.
const latencyBuckets = 63

// latBucket returns the histogram bucket index for a nanosecond latency value.
func latBucket(ns int64) int {
	if ns <= 1 {
		return 0
	}

	b := bits.Len64(uint64(ns)) - 1
	if b >= latencyBuckets {
		b = latencyBuckets - 1
	}

	return b
}

// latPercentile returns the p-th percentile (0–100) from a log2 latency histogram.
// The result is the lower bound of the bucket that contains the p-th observation.
// Returns 0 when no positive-latency observations have been recorded.
func latPercentile(hist *[latencyBuckets]int64, p float64) time.Duration {
	var total int64

	for _, c := range hist {
		total += c
	}

	if total == 0 {
		return 0
	}

	target := total * int64(p) / 100
	if target == 0 {
		target = 1
	}

	var acc int64

	for i := range hist {
		acc += hist[i]
		if acc >= target {
			return time.Duration(int64(1) << uint(i))
		}
	}

	return time.Duration(int64(1) << 62)
}
