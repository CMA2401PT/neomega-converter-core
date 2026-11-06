package converter

import (
	"fmt"

	"celler"
	"neomega-blocks/ir"
)

type DiagnosticKind string

// 诊断种类让调用方分别汇总映射缺失、NBT 解码和实体规则问题。

const (
	UnknownBlock           DiagnosticKind = "unknown_block"
	NBTError               DiagnosticKind = "nbt_error"
	RuleError              DiagnosticKind = "rule_error"
	UnsupportedBlockEntity DiagnosticKind = "unsupported_block_entity"
	UnknownEntity          DiagnosticKind = "unknown_entity"
	UnknownItem            DiagnosticKind = "unknown_item"
)

type Diagnostic struct {
	// 可恢复的问题随区块一起返回；合并计数的未知状态可以没有单格坐标。
	Kind          DiagnosticKind
	Pos           *celler.BlockPos
	State         ir.IRBlock
	Count         uint64
	Err           error
	BlockEntityID string
	EntityID      string
	ItemID        string
}

// ReportUnknownItem 合并同一物品名的失败计数。
// 区块转换是单 goroutine 的；结果对象不跨任务共享。
func (c *JavaChunk) ReportUnknownItem(name string) {
	c.Diagnostics = reportUnknownItem(c.Diagnostics, name)
}

// ReportUnknownItem 是 Java → Bedrock 方向的同名入口。
func (c *BedrockChunk) ReportUnknownItem(name string) {
	c.Diagnostics = reportUnknownItem(c.Diagnostics, name)
}

func reportUnknownItem(diagnostics []Diagnostic, name string) []Diagnostic {
	for i := range diagnostics {
		if diagnostics[i].Kind == UnknownItem && diagnostics[i].ItemID == name {
			diagnostics[i].Count++
			return diagnostics
		}
	}
	return append(diagnostics, Diagnostic{
		Kind:   UnknownItem,
		ItemID: name,
		Count:  1,
		Err:    fmt.Errorf("unknown or invalid item: %q", name),
	})
}
