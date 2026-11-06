// Package bedrockout 把 converter.BedrockChunk 写成 Bedrock 1.21.50 的原生记录。
// 一根 Bedrock 柱子在数据库里长什么样，只由这里决定；Java 存档转换和 mcstructure 转换都经过它。
package bedrockout

import (
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-converter-core"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

// EncodeChunk 把转换结果编码为独占的 Bedrock 原生记录，不访问数据库，可在独立任务中调用。
func EncodeChunk(chunk *converter.BedrockChunk) (*cellerbedrock.EncodedChunk, error) {
	target := cellerbedrock.NewChunk(chunk.Pos, chunk.Dimension)
	for y, section := range chunk.Sections {
		layers := make([]*cellerbedrock.Layer, len(section.Layers))
		for index, palette := range section.Layers {
			layers[index] = &cellerbedrock.Layer{Blocks: palette}
		}
		if err := target.SetSection(&cellerbedrock.Section{Y: y, Version: sectionVersion, Layers: layers}); err != nil {
			return nil, err
		}
	}
	if len(chunk.BlockEntities) > 0 {
		documents := make([]nbt.Document, 0, len(chunk.BlockEntities))
		for _, entity := range chunk.BlockEntities {
			documents = append(documents, nbt.Document{Value: entity})
		}
		if err := target.SetBlockEntities(documents); err != nil {
			return nil, err
		}
	}
	var actors []cellerbedrock.Actor
	if len(chunk.Actors) > 0 {
		actors = make([]cellerbedrock.Actor, len(chunk.Actors))
		for index, actor := range chunk.Actors {
			actors[index] = cellerbedrock.Actor{ID: actor.ID, NBT: nbt.Document{Value: actor.NBT}}
		}
	}
	if biomes, ok := biomes3D(chunk); ok {
		if err := target.SetBiomes3D(biomes); err != nil {
			return nil, err
		}
	}
	// 现代 Bedrock 的区块版本与完成标记；它们决定游戏是否重新生成这个柱子。
	target.Records[string([]byte{cellerbedrock.TagVersion})] = []byte{chunkVersion}
	target.Records[string([]byte{cellerbedrock.TagFinalizedState})] = []byte{2}
	return cellerbedrock.EncodeChunk(target, actors)
}

// WriteChunk 编码并替换一根 Bedrock 柱子，地形和实体在同一个批次中提交。
func WriteChunk(world *cellerbedrock.World, chunk *converter.BedrockChunk) error {
	encoded, err := EncodeChunk(chunk)
	if err != nil {
		return err
	}
	return world.WriteChunks([]*cellerbedrock.EncodedChunk{encoded}, cellerbedrock.WriteChunksOptions{})
}

// biomes3D 从每个 section 的 16³ 群系调色板拼出 Data3D。
// Bedrock 的第一个群系 section 从维度最低段开始，缺失的段写 plains。
func biomes3D(chunk *converter.BedrockChunk) (*cellerbedrock.Biomes3D, bool) {
	if len(chunk.Sections) == 0 {
		return nil, false
	}
	low, high := sectionBounds(chunk.Dimension)
	out := &cellerbedrock.Biomes3D{}
	for y := low; y <= high; y++ {
		section := chunk.Sections[y]
		if section == nil || section.Biomes == nil {
			out.Sections = append(out.Sections, cellerbedrock.BiomeSection{Values: plainsPalette()})
			continue
		}
		out.Sections = append(out.Sections, cellerbedrock.BiomeSection{Values: section.Biomes})
	}
	return out, true
}

func plainsPalette() *layer_storage.Palette[int32] {
	id := int32(1)
	if value, ok := tables.BedrockBiome("minecraft:plains"); ok {
		id = value
	}
	return layer_storage.MakePaletteStorage(id)
}

// sectionBounds 是原版维度的建筑高度：主世界 -4..19，下界和末地 0..15。
func sectionBounds(dimension int32) (int32, int32) {
	if dimension == 0 {
		return -4, 19
	}
	return 0, 15
}

// 目标 Bedrock 版本的 section 与区块记录版本。
const (
	sectionVersion = 9
	chunkVersion   = 42
)
