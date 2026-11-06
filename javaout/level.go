package javaout

import nbt "neomega-nbt/presets/v1"

// NewLevelData 返回一个原版新世界 level.dat 的 Data：版本字段，加上新世界的默认值。
// 调用方只覆盖自己知道的字段；Bedrock 存档转换覆盖源世界里有的，结构转换只写名称和出生点。
func NewLevelData() nbt.Compound {
	return nbt.Compound{
		"DataVersion": DataVersion,
		"version":     int32(19133),
		"Version":     nbt.Compound{"Id": DataVersion, "Name": VersionName, "Series": "main", "Snapshot": int8(0)},
		"initialized": int8(1),
		"DataPacks":   nbt.Compound{"Enabled": []string{"vanilla"}, "Disabled": []string{}},

		"LevelName":        "New World",
		"SpawnX":           int32(0),
		"SpawnY":           int32(64),
		"SpawnZ":           int32(0),
		"SpawnAngle":       float32(0),
		"GameType":         int32(0),
		"Difficulty":       int8(2),
		"Time":             int64(0),
		"DayTime":          int64(0),
		"allowCommands":    int8(0),
		"hardcore":         int8(0),
		"LastPlayed":       int64(0),
		"raining":          int8(0),
		"thundering":       int8(0),
		"rainTime":         int32(0),
		"thunderTime":      int32(0),
		"clearWeatherTime": int32(0),
		"GameRules":        nbt.Compound{},
		"WorldGenSettings": WorldGenSettings(0, true, nil),
	}
}

// Level 把 Data 包成 level.dat 文档。
func Level(data nbt.Compound) nbt.Document {
	return nbt.Document{Value: nbt.Compound{"Data": data}}
}

// WorldGenSettings 写原版三维度的生成器。1.21.4 把生成配置放在 Data 里，不是 26.x 的独立文件。
// overworld 不为 nil 时替换主世界的生成器，例如 FlatGenerator 的结果。
func WorldGenSettings(seed int64, generateFeatures bool, overworld nbt.Compound) nbt.Compound {
	main := noiseDimension("overworld", "overworld", "overworld")
	if overworld != nil {
		main["generator"] = overworld
	}
	generate := int8(0)
	if generateFeatures {
		generate = 1
	}
	return nbt.Compound{
		"seed": seed, "generate_features": generate, "bonus_chest": int8(0),
		"dimensions": nbt.Compound{
			"minecraft:overworld":  main,
			"minecraft:the_nether": noiseDimension("the_nether", "nether", "nether"),
			"minecraft:the_end":    noiseDimension("the_end", "end", ""),
		},
	}
}

// FlatLayer 是超平坦的一层：Java 方块名与层厚。
type FlatLayer struct {
	Block  string
	Height int32
}

// FlatGenerator 返回超平坦生成器；layers 为空就是一个虚空世界。
func FlatGenerator(biome string, layers []FlatLayer) nbt.Compound {
	list := make([]nbt.Compound, 0, len(layers))
	for _, layer := range layers {
		list = append(list, nbt.Compound{"block": layer.Block, "height": layer.Height})
	}
	return nbt.Compound{
		"type": "minecraft:flat",
		"settings": nbt.Compound{
			"biome": biome, "features": int8(0), "lakes": int8(0),
			"layers": list, "structure_overrides": []string{},
		},
	}
}

func noiseDimension(kind, settings, preset string) nbt.Compound {
	// 三套原版生成器只引用内置注册表，继续生成新区块时不需要附带数据包。
	biome := nbt.Compound{"type": "minecraft:multi_noise", "preset": "minecraft:" + preset}
	if preset == "" {
		biome = nbt.Compound{"type": "minecraft:the_end"}
	}
	return nbt.Compound{
		"type":      "minecraft:" + kind,
		"generator": nbt.Compound{"type": "minecraft:noise", "settings": "minecraft:" + settings, "biome_source": biome},
	}
}
