package aggregator

import "sort"

const (
	fileStatsCap = 10000
)

// FileStat represents the count of syscalls attributed to a specific file path.
// This function is used to return the top files in the TopFiles and TopFilesForSyscall methods.
func topFilesFromMap(m map[string]int64, n int) []FileStat {
	if m == nil {
		return nil
	}

	// Convert the map to a slice of FileStat for sorting.
	out := make([]FileStat, 0, len(m))
	for p, c := range m {
		out = append(out, FileStat{Path: p, Count: c})
	}

	// Sort the slice by count in descending order and apply the limit if n > 0.
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if n > 0 && len(out) > n {
		out = out[:n]
	}

	return out
}
