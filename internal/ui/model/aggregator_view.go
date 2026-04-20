package model

import (
	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

// AggregatorView is a small interface that the UI model depends on.
// It intentionally mirrors a subset of methods provided by aggregator.Aggregator
// so the UI can remain decoupled from the concrete implementation.
type AggregatorView interface {
	Total() int64
	Errors() int64
	Rate() float64
	UniqueCount() int
	Sorted(aggregator.SortField) []aggregator.SyscallStat
	RecentLog() []aggregator.LogEntry
	TopFiles(n int) []aggregator.FileStat
	CategoryBreakdown() map[aggregator.Category]aggregator.CategoryStats
	GetProcInfo() procinfo.ProcInfo
	IsPerPID() bool
}
