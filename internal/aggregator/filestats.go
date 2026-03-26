package aggregator

import "sort"

const (
	fileStatsCap = 10000
)

func topFilesFromMap(m map[string]int64, n int) []FileStat {
	if m == nil {
		return nil
	}
	out := make([]FileStat, 0, len(m))
	for p, c := range m {
		out = append(out, FileStat{Path: p, Count: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}
