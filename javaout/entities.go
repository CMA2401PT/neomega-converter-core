package javaout

import (
	"neomega-converter-core"
	nbt "neomega-nbt/presets/v1"
)

// Entities 返回柱子的实体文档；Java 把实体存在独立于地形的 region 里。
// 没有实体时返回 nil，调用方直接跳过这条记录。
func Entities(chunk *converter.JavaChunk) *nbt.Document {
	if len(chunk.Entities) == 0 {
		return nil
	}
	return &nbt.Document{Value: nbt.Compound{
		"DataVersion": DataVersion,
		"Position":    nbt.Array([]int32{chunk.Pos.X, chunk.Pos.Z}),
		"Entities":    chunk.Entities,
	}}
}
