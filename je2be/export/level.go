package export

import (
	"encoding/json"

	cellerbedrock "celler/bedrock"
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter-core/bedrockout"
	"neomega-converter-core/mapping"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

// LevelReport 记录源数据缺失、改用 Bedrock 默认值的字段。
type LevelReport struct {
	DefaultedFields []string
}

// ConvertLevel 把 Java 的 level.dat 转成 Bedrock 的 level.dat。
func ConvertLevel(document nbt.Document) (cellerbedrock.Level, LevelReport) {
	data, _ := document.Value["Data"].(nbt.Compound)
	report := LevelReport{}
	root := bedrockout.NewLevelData()
	root["LevelName"] = value(data, "LevelName", "New World", &report)
	root["SpawnX"] = value(data, "SpawnX", int32(0), &report)
	root["SpawnY"] = value(data, "SpawnY", int32(64), &report)
	root["SpawnZ"] = value(data, "SpawnZ", int32(0), &report)
	if angle, ok := floatOf(data["SpawnAngle"]); ok {
		root["SpawnRotation"] = float32(angle)
	}
	root["GameType"] = int32(number(data, "GameType", 0, &report))
	root["Difficulty"] = int32(number(data, "Difficulty", 2, &report))
	root["IsHardcore"] = flag(data["hardcore"])
	root["commandsEnabled"] = flag(data["allowCommands"])
	// Java 的 Time 是总时间，DayTime 是昼夜时间；Bedrock 的字段名正好相反。
	gameTime := number(data, "Time", 0, &report)
	dayTime := number(data, "DayTime", gameTime, &report)
	root["currentTick"] = gameTime
	root["Time"] = dayTime
	root["LastPlayed"] = number(data, "LastPlayed", 0, &report) / 1000
	root["rainLevel"] = ratio(data["raining"])
	root["lightningLevel"] = ratio(data["thundering"])
	root["rainTime"] = int32(number(data, "rainTime", 0, &report))
	root["lightningTime"] = int32(number(data, "thunderTime", 0, &report))

	for _, rule := range mapping.GameRules {
		source, exists := gameRules(data)[rule.Java]
		if !exists {
			continue
		}
		switch source := source.(type) {
		case string:
			root[rule.Bedrock] = boolByte(source == "true")
		case int32:
			root[rule.Bedrock] = source
		}
	}

	root["Generator"] = int32(1)
	root["GenerateStructures"] = int8(1)
	if flat, ok := flatWorld(data); ok {
		root["Generator"] = int32(2)
		root["FlatWorldLayers"] = flat
	}
	if settings, ok := data["WorldGenSettings"].(nbt.Compound); ok {
		if seed, ok := numberOf(settings["seed"]); ok {
			root["RandomSeed"] = seed
		} else {
			report.DefaultedFields = append(report.DefaultedFields, "RandomSeed")
		}
		if generate, ok := numberOf(settings["generate_features"]); ok {
			root["GenerateStructures"] = boolByte(generate != 0)
		}
	} else {
		report.DefaultedFields = append(report.DefaultedFields, "RandomSeed")
	}
	return bedrockout.Level(root), report
}

func gameRules(data nbt.Compound) nbt.Compound {
	rules, _ := data["GameRules"].(nbt.Compound)
	if rules == nil {
		return nbt.Compound{}
	}
	return rules
}

type flatLayer struct {
	BlockName string `json:"block_name"`
	Count     int32  `json:"count"`
}

type flatWorldData struct {
	BiomeID     int32       `json:"biome_id"`
	BlockLayers []flatLayer `json:"block_layers"`
}

// flatWorld 把 Java 的 flat 生成器还原成 Bedrock 的 FlatWorldLayers JSON。
func flatWorld(data nbt.Compound) (string, bool) {
	settings, ok := data["WorldGenSettings"].(nbt.Compound)
	if !ok {
		return "", false
	}
	dimensions, ok := settings["dimensions"].(nbt.Compound)
	if !ok {
		return "", false
	}
	overworld, ok := dimensions["minecraft:overworld"].(nbt.Compound)
	if !ok {
		return "", false
	}
	generator, ok := overworld["generator"].(nbt.Compound)
	if !ok || generator["type"] != "minecraft:flat" {
		return "", false
	}
	flat, ok := generator["settings"].(nbt.Compound)
	if !ok {
		return "", false
	}
	out := flatWorldData{}
	if biome, ok := flat["biome"].(string); ok {
		if id, ok := tables.BedrockBiome(biome); ok {
			out.BiomeID = id
		}
	}
	layers, _ := nbt.Compounds(flat["layers"])
	for _, layer := range layers {
		java, _ := layer["block"].(string)
		name := java
		if state, ok := converts.ConvertFromJava(ir.IRBlock(java + " []")); ok {
			name = state.NameWithNameSpace()
		}
		height, _ := numberOf(layer["height"])
		out.BlockLayers = append(out.BlockLayers, flatLayer{BlockName: name, Count: int32(height)})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func value[T any](data nbt.Compound, key string, fallback T, report *LevelReport) T {
	if value, ok := data[key].(T); ok {
		return value
	}
	report.DefaultedFields = append(report.DefaultedFields, key)
	return fallback
}

func number(data nbt.Compound, key string, fallback int64, report *LevelReport) int64 {
	if value, ok := numberOf(data[key]); ok {
		return value
	}
	report.DefaultedFields = append(report.DefaultedFields, key)
	return fallback
}

func numberOf(value any) (int64, bool) {
	switch value := value.(type) {
	case int8:
		return int64(value), true
	case int16:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	default:
		return 0, false
	}
}

func floatOf(value any) (float64, bool) {
	switch value := value.(type) {
	case float32:
		return float64(value), true
	case float64:
		return value, true
	default:
		return 0, false
	}
}

func flag(value any) int8 {
	if number, ok := numberOf(value); ok && number != 0 {
		return 1
	}
	return 0
}

func ratio(value any) float32 {
	if number, ok := numberOf(value); ok && number > 0 {
		return 1
	}
	return 0
}

func boolByte(value bool) int8 {
	if value {
		return 1
	}
	return 0
}
