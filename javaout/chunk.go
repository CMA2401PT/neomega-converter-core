// Package javaout 把 converter.JavaChunk 写成 Java 1.21.4 的原生记录。
// 一根 Java 柱子在存档里长什么样，只由这里决定；Bedrock 存档转换和结构文件转换都经过它。
package javaout

import (
	"sort"
	"strings"

	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-converter-core"
	nbt "neomega-nbt/presets/v1"
)

// 目标状态表对应 Java 1.21.4，区块、实体和世界元数据必须使用同一版本。
const (
	DataVersion int32 = 4189
	VersionName       = "1.21.4"
)

// Height 是写出的 section 闭区间。Low 同时是区块的 yPos，PostProcessing 的下标也从它算起。
type Height struct {
	Low, High int32
}

// DimensionHeight 返回原版维度的建筑高度：主世界 -4..19，下界和末地 0..15。
// 源世界可能有更高的柱子，超出部分在写出时丢弃。
func DimensionHeight(dimension int32) Height {
	if dimension == 0 {
		return Height{Low: -4, High: 19}
	}
	return Height{Low: 0, High: 15}
}

// Dimension 把 Bedrock 的维度编号映射为 Java 的维度目录。
// 本仓库的中间结果统一使用 Bedrock 编号 0/1/2。
func Dimension(dimension int32) (cellerjava.Dimension, bool) {
	switch dimension {
	case 0:
		return cellerjava.Overworld, true
	case 1:
		return cellerjava.Nether, true
	case 2:
		return cellerjava.End, true
	default:
		return "", false
	}
}

// Chunk 按 height 写出柱子，只补加载所需的容器字段，不改变方块状态。
func Chunk(chunk *converter.JavaChunk, height Height) (*cellerjava.Chunk, error) {
	low, high := height.Low, height.High
	result := cellerjava.NewChunk(chunk.Pos, DataVersion)
	ys := make([]int32, 0, len(chunk.Sections))
	for y := range chunk.Sections {
		if y >= low && y <= high {
			ys = append(ys, y)
		}
	}
	sort.Slice(ys, func(i, j int) bool { return ys[i] < ys[j] })
	// Java 按最低 section 解释列表下标，缺失的 section 也必须保留空列表占位。
	post := make([][]int16, high-low+1)
	for i := range post {
		post[i] = []int16{}
	}
	for _, y := range ys {
		source := chunk.Sections[y]
		if err := result.SetSection(&cellerjava.Section{
			Y:      y,
			Blocks: source.Blocks,
			Biomes: source.Biomes,
		}); err != nil {
			return nil, err
		}
		if source.Blocks == nil {
			continue
		}
		palette := source.Blocks.GetUsedPalette()
		needs := make([]bool, len(palette))
		anyNeeds := false
		for i, block := range palette {
			needs[i] = needsShapeUpdate(block.NameWithNameSpace())
			anyNeeds = anyNeeds || needs[i]
		}
		if !anyNeeds {
			continue
		}
		for index := range 4096 {
			x, ly, z := index&15, index>>8, index>>4&15
			if needs[source.Blocks.GetIndex(layer_storage.At(x, ly, z))] {
				post[y-low] = append(post[y-low], int16(x|ly<<4|z<<8))
			}
		}
	}
	entities := make([]nbt.Compound, 0, len(chunk.BlockEntities))
	for _, entity := range chunk.BlockEntities {
		if entity.Pos.Y>>4 >= low && entity.Pos.Y>>4 <= high {
			entities = append(entities, entity.NBT)
		}
	}
	root := result.NBT.Value
	root["yPos"] = low
	root["Status"] = "minecraft:full"
	root["LastUpdate"], root["InhabitedTime"] = int64(0), int64(0)
	root["isLightOn"] = int8(0)
	root["Heightmaps"] = nbt.Compound{}
	root["PostProcessing"] = post
	root["block_entities"] = entities
	root["block_ticks"], root["fluid_ticks"] = []nbt.Compound{}, []nbt.Compound{}
	root["structures"] = nbt.Compound{"References": nbt.Compound{}, "starts": nbt.Compound{}}
	return result, nil
}

func needsShapeUpdate(name string) bool {
	// 只登记不会因缺支撑而被邻接重算拆掉的形状方块；登记全部方块会使悬空装饰掉落。
	name = strings.TrimPrefix(name, "minecraft:")
	return strings.HasSuffix(name, "_fence") || strings.HasSuffix(name, "_fence_gate") ||
		name == "glass_pane" || strings.HasSuffix(name, "_stained_glass_pane") || name == "iron_bars" ||
		strings.HasSuffix(name, "_stairs") || strings.HasSuffix(name, "_wall") ||
		name == "redstone_wire" || name == "tripwire" || name == "chorus_plant"
}
