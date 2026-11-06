package export

import (
	"encoding/json"
	"reflect"

	cellerbedrock "celler/bedrock"
	"neomega-blocks/ir"
	"neomega-converter-core/be2je"
	"neomega-converter-core/javaout"
	"neomega-converter-core/mapping"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

type LevelReport struct {
	// 默认字段单独报告，避免将源世界缺失的信息伪装成成功保留。
	DefaultedFields []string
}

// ConvertLevel 把 Bedrock 的 level.dat 转成 Java 1.21.4 的 level.dat。
// 只取两版含义一致的字段；取不到或类型不对时保留 Java 新世界的值，并记进报告。
func ConvertLevel(level cellerbedrock.Level) (nbt.Document, LevelReport) {
	source := level.NBT.Value
	data := javaout.NewLevelData()
	report := LevelReport{}
	copyField(data, "LevelName", source, "LevelName", &report)
	copyField(data, "SpawnX", source, "SpawnX", &report)
	copyField(data, "SpawnY", source, "SpawnY", &report)
	copyField(data, "SpawnZ", source, "SpawnZ", &report)
	copyField(data, "SpawnAngle", source, "SpawnRotation", &report)
	seed := levelValue(source, "RandomSeed", int64(0), &report)
	copyField(data, "GameType", source, "GameType", &report)
	// Bedrock 的 Difficulty 是 Int，Java 用 Byte；Time 和 currentTick 分别对应昼夜与总时间。
	if difficulty, ok := source["Difficulty"].(int32); ok {
		data["Difficulty"] = int8(difficulty)
	} else {
		report.DefaultedFields = append(report.DefaultedFields, "Difficulty")
	}
	dayTime := levelValue(source, "Time", int64(0), &report)
	gameTime := levelValue(source, "currentTick", dayTime, &report)
	data["Time"], data["DayTime"] = gameTime, dayTime
	copyField(data, "allowCommands", source, "commandsAllowed", &report)
	data["hardcore"] = flag(source["IsHardcore"])
	// Bedrock 的 LastPlayed 是秒，Java 是毫秒。
	data["LastPlayed"] = levelValue(source, "LastPlayed", int64(0), &report) * 1000
	data["raining"] = ratio(source["rainLevel"])
	data["thundering"] = ratio(source["lightningLevel"])
	copyField(data, "rainTime", source, "rainTime", &report)
	copyField(data, "thunderTime", source, "lightningTime", &report)
	data["GameRules"] = convertGameRules(source)

	generate := true
	if value, ok := numberOf(source["GenerateStructures"]); ok {
		generate = value != 0
	}
	overworld, _ := flatGenerator(source)
	data["WorldGenSettings"] = javaout.WorldGenSettings(seed, generate, overworld)
	return javaout.Level(data), report
}

// copyField 复制两版类型相同的字段；类型以 data 里的默认值为准。
func copyField(data nbt.Compound, javaKey string, source nbt.Compound, bedrockKey string, report *LevelReport) {
	if value, ok := source[bedrockKey]; ok && reflect.TypeOf(value) == reflect.TypeOf(data[javaKey]) {
		data[javaKey] = value
		return
	}
	report.DefaultedFields = append(report.DefaultedFields, bedrockKey)
}

func levelValue[T any](source nbt.Compound, key string, fallback T, report *LevelReport) T {
	// 类型不符也采用目标默认值，所有替代均从这条路径进入报告。
	if value, ok := source[key].(T); ok {
		return value
	}
	report.DefaultedFields = append(report.DefaultedFields, key)
	return fallback
}

// flag 接受 Bedrock 不同宽度的布尔字段，返回 Java 的 Byte。
func flag(value any) int8 {
	number, ok := numberOf(value)
	if !ok || number == 0 {
		return 0
	}
	return 1
}

// ratio 把 Bedrock 的 0..1 强度字段变成 Java 的布尔值。
func ratio(value any) int8 {
	number, ok := realOf(value)
	if !ok || number <= 0 {
		return 0
	}
	return 1
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

func realOf(value any) (float64, bool) {
	switch value := value.(type) {
	case float32:
		return float64(value), true
	case float64:
		return value, true
	default:
		return 0, false
	}
}

func convertGameRules(source nbt.Compound) nbt.Compound {
	rules := nbt.Compound{}
	for _, rule := range mapping.GameRules {
		value, exists := source[rule.Bedrock]
		if !exists {
			continue
		}
		switch value := value.(type) {
		case int8:
			if value != 0 {
				rules[rule.Java] = "true"
			} else {
				rules[rule.Java] = "false"
			}
		case int32:
			rules[rule.Java] = value
		}
	}
	return rules
}

type flatLayers struct {
	BiomeID     int32 `json:"biome_id"`
	BlockLayers []struct {
		BlockName string `json:"block_name"`
		Count     int32  `json:"count"`
	} `json:"block_layers"`
}

// flatGenerator 把 Bedrock 的平坦世界设置转成 Java 的 flat 生成器。
// 解析失败时返回 false，调用方退回普通噪声生成器。
func flatGenerator(source nbt.Compound) (nbt.Compound, bool) {
	generator, ok := numberOf(source["Generator"])
	if !ok || generator != 2 {
		return nil, false
	}
	raw, ok := source["FlatWorldLayers"].(string)
	if !ok {
		return nil, false
	}
	var flat flatLayers
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, false
	}
	layers := make([]javaout.FlatLayer, 0, len(flat.BlockLayers))
	for _, layer := range flat.BlockLayers {
		name := namespaced(layer.BlockName)
		if state, ok := be2je.ConvertState(ir.IRBlock(name + " []")); ok {
			name = state.NameWithNameSpace()
		}
		layers = append(layers, javaout.FlatLayer{Block: name, Height: layer.Count})
	}
	biome := "minecraft:plains"
	if java, ok := tables.Biome(flat.BiomeID); ok {
		biome = java
	}
	return javaout.FlatGenerator(biome, layers), true
}

func namespaced(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] == ':' {
			return name
		}
	}
	return "minecraft:" + name
}
