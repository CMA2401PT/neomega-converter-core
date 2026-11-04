package converter

import (
	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"

	nbt "neomega-nbt/presets/v1"
)

// BedrockChunk 是 Java → Bedrock 的中间结果。
// 它和 ConvertedChunk 分开：两个字段里的 ir 方块状态属于不同的 hash 空间。
type BedrockChunk struct {
	Pos       celler.ChunkPos
	Dimension int32
	Sections  map[int32]*BedrockSection
	// BlockEntities 每项是一个带 id/x/y/z 的 Bedrock 方块实体 Compound。
	BlockEntities []nbt.Compound
	Actors        []BedrockActor
	Diagnostics   []Diagnostic
}

// BedrockActor 是一条待写入 digp/actorprefix 的实体：ID 是 8 字节键。
type BedrockActor struct {
	ID  [8]byte
	NBT nbt.Compound
}

type BedrockSection struct {
	// Layers[0] 是基础方块；有含水方块时 Layers[1] 是水层。
	Layers []*layer_storage.Palette[cellerbedrock.Block]
	// Biomes 是 Bedrock 的 16×16×16 群系编号。
	Biomes *layer_storage.Palette[int32]
}
