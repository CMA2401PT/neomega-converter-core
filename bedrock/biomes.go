package bedrock

import (
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-converter-core"
	"neomega-converter-core/third_party"
)

// 没有群系数据的 section 统一写 plains，避免 Java 区块缺少必需的 biomes 字段。
// 所有 section 共享同一份只读数组。
var plainsBiomes = func() *[64]string {
	var biomes [64]string
	for i := range biomes {
		biomes[i] = "minecraft:plains"
	}
	return &biomes
}()

// applyBiomes 把 Bedrock 的群系记录填进转换结果。Data3D 优先，2D 回退。
func applyBiomes(src *cellerbedrock.Chunk, result *converter.ConvertedChunk) {
	if data, err := src.Biomes3D(); err == nil {
		applyBiomes3D(data, result)
		return
	}
	if data, err := src.Biomes2D(); err == nil {
		applyBiomes2D(data, result)
	}
}

// biomeMinSection 是 Data3D 第一个 section 对应的世界 section Y。
// 主世界从 -64 开始，下界和末地从 0 开始，和 mcconv 的 Bounds 一致。
func biomeMinSection(dimension int32) int32 {
	if dimension == 0 {
		return -4
	}
	return 0
}

func applyBiomes3D(data *cellerbedrock.Biomes3D, result *converter.ConvertedChunk) {
	base := biomeMinSection(result.Dimension)
	var previous *[64]string
	for index, section := range data.Sections {
		target := result.Sections[base+int32(index)]
		if section.Values == nil {
			// 继承上一条记录；没有上一条时保留默认 plains。
			if target != nil && previous != nil {
				target.Biomes = previous
			}
			continue
		}
		biomes := new([64]string)
		for y4 := 0; y4 < 4; y4++ {
			for z4 := 0; z4 < 4; z4++ {
				for x4 := 0; x4 < 4; x4++ {
					// Bedrock 的 16³ 群系数组按 4×4×4 取样，Java 每 section 只要 64 个值。
					id := section.Values.Get(layer_storage.At(x4*4, y4*4, z4*4))
					biomes[y4<<4|z4<<2|x4] = biomeName(id)
				}
			}
		}
		previous = biomes
		if target != nil {
			target.Biomes = biomes
		}
	}
}

func applyBiomes2D(data cellerbedrock.Biomes2D, result *converter.ConvertedChunk) {
	// 2D 记录只有每个 16×16 列一个值；每个 4×4 单元取列角落的样本，
	// 和 mcconv 的做法一致。所有 section 共用同一份 4×4×4 结果。
	biomes := new([64]string)
	for y4 := 0; y4 < 4; y4++ {
		for z4 := 0; z4 < 4; z4++ {
			for x4 := 0; x4 < 4; x4++ {
				id := int32(data.Values[(z4*4)*16+x4*4])
				biomes[y4<<4|z4<<2|x4] = biomeName(id)
			}
		}
	}
	for _, section := range result.Sections {
		section.Biomes = biomes
	}
}

func biomeName(id int32) string {
	if java, ok := tables.Biome(id); ok {
		return java
	}
	// 未知编号按原版回退，和 mcconv 的 plains 兜底一致。
	return "minecraft:plains"
}
