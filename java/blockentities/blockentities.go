// Package blockentities 定义 Java → Bedrock 的方块实体规则接口。
// 和 bedrock/blockentities 分开：这里的 Source 是 Java NBT，Base 是 Bedrock 状态。
package blockentities

import (
	"celler"
	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
)

// StateString 读取字符串属性；Java 和 Bedrock 的 IR 都用字符串保存大部分枚举。
func StateString(state ir.IRBlock, key string) (string, bool) {
	value, found := state.States().Get(key)
	if !found {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

type Input struct {
	// SourceState 是 Java 源状态，规则用它读取 Java 专属属性（例如箱子的 type）。
	SourceState ir.IRBlock
	// Base 是已经转出的 Bedrock 状态，规则通常原样返回。
	Base ir.IRBlock
	// Source 是 Java 方块实体 NBT，包含 id/x/y/z。
	Source    nbt.Compound
	WorldPos  celler.BlockPos
	Dimension int32
	// ReportItem 在物品转不出时收到 Java 物品 id，可以为 nil。
	ReportItem func(name string)
}

type Result struct {
	// State 是写回调色板的 Bedrock 状态；零值表示沿用 Base。
	State ir.IRBlock
	// BlockEntity 是 Bedrock 方块实体 NBT，需要自己带 id，入口补 x/y/z。
	BlockEntity nbt.Compound
}

type ConvertFunc func(Input) (Result, error)
