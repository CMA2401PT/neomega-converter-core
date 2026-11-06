package converter

import (
	"celler"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"

	nbt "neomega-nbt/presets/v1"
)

// JavaChunk 是写成 Java 存档前的一根柱子：Java 方块状态、方块实体与实体。
// be2je 和 Java 结构文件产出它，javaout 把它写成 region 记录；缓存和邻居修复也共享这一份数据。
type JavaChunk struct {
	Pos           celler.ChunkPos
	Dimension     int32
	Sections      map[int32]*JavaSection
	BlockEntities []cellerjava.BlockEntity
	Diagnostics   []Diagnostic
	Entities      []nbt.Compound
}

type JavaSection struct {
	// 方块按调色板共享状态；生物群系尚无映射时用 nil 表达缺失。
	Blocks *layer_storage.Palette[ir.IRBlock]
	Biomes *[64]string
}
