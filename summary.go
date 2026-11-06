package converter

import (
	"cmp"
	"slices"

	"celler"
	"neomega-blocks/ir"
)

// FailedChunk 是整图转换中没有写出的柱子；错误与位置一起保存，调用方可以定位它。
type FailedChunk struct {
	Dimension int32
	Pos       celler.ChunkPos
	Err       error
}

// DiagnosticSummary 把整图转换逐柱返回的诊断合并成计数。
// 两个方向的整图转换共用它，补映射时可以用同一套字段对比两边缺什么。
type DiagnosticSummary struct {
	// 合并计数的诊断按 Count 计，单格诊断按 1 计。
	ByKind map[DiagnosticKind]uint64
	// 以下三项按名称细分，计数含义同 ByKind。
	UnknownStates            map[ir.IRBlock]uint64
	UnknownItems             map[string]uint64
	UnsupportedBlockEntities map[string]uint64
}

func NewDiagnosticSummary() DiagnosticSummary {
	return DiagnosticSummary{
		ByKind:                   make(map[DiagnosticKind]uint64),
		UnknownStates:            make(map[ir.IRBlock]uint64),
		UnknownItems:             make(map[string]uint64),
		UnsupportedBlockEntities: make(map[string]uint64),
	}
}

// Add 计入一根柱子的诊断。整图转换只在写出侧的单个 goroutine 里调用它。
func (s *DiagnosticSummary) Add(diagnostics []Diagnostic) {
	for _, diagnostic := range diagnostics {
		count := max(diagnostic.Count, 1)
		s.ByKind[diagnostic.Kind] += count
		switch diagnostic.Kind {
		case UnknownBlock:
			s.UnknownStates[diagnostic.State] += count
		case UnknownItem:
			s.UnknownItems[diagnostic.ItemID] += count
		case UnsupportedBlockEntity:
			s.UnsupportedBlockEntities[diagnostic.BlockEntityID] += count
		}
	}
}

// Count 是 TopCounts 返回的一项。
type Count[K cmp.Ordered] struct {
	Key   K
	Count uint64
}

// TopCounts 按计数从大到小取前 n 项，计数相同时按名称排序，输出稳定便于前后对比。
// 补映射时先补影响最大的几项，所以报告通常只打印前十。
func TopCounts[K cmp.Ordered](counts map[K]uint64, n int) []Count[K] {
	list := make([]Count[K], 0, len(counts))
	for key, count := range counts {
		list = append(list, Count[K]{Key: key, Count: count})
	}
	slices.SortFunc(list, func(a, b Count[K]) int {
		if a.Count != b.Count {
			return cmp.Compare(b.Count, a.Count)
		}
		return cmp.Compare(a.Key, b.Key)
	})
	return list[:min(n, len(list))]
}
