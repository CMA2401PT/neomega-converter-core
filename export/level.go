package export

import (
	"encoding/json"

	cellerbedrock "celler/bedrock"
	"neomega-blocks/ir"
	"neomega-converter-core/bedrock"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

type LevelReport struct {
	// 默认字段单独报告，避免将源世界缺失的信息伪装成成功保留。
	DefaultedFields []string
}

// gameRules 是 Bedrock level.dat 顶层键到 Java 1.21.4 规则名的对照。
// 只列两版都存在的规则；Bedrock 独有或 Java 独有的规则不在这里。
var gameRules = []struct{ bedrock, java string }{
	{"dodaylightcycle", "doDaylightCycle"},
	{"doweathercycle", "doWeatherCycle"},
	{"dofiretick", "doFireTick"},
	{"doinsomnia", "doInsomnia"},
	{"domobspawning", "doMobSpawning"},
	{"domobloot", "doMobLoot"},
	{"dotiledrops", "doTileDrops"},
	{"doentitydrops", "doEntityDrops"},
	{"dolimitedcrafting", "doLimitedCrafting"},
	{"doimmediaterespawn", "doImmediateRespawn"},
	{"keepinventory", "keepInventory"},
	{"mobgriefing", "mobGriefing"},
	{"naturalregeneration", "naturalRegeneration"},
	{"randomtickspeed", "randomTickSpeed"},
	{"playerssleepingpercentage", "playersSleepingPercentage"},
	{"projectilescanbreakblocks", "projectilesCanBreakBlocks"},
	{"pvp", "pvp"},
	{"spawnradius", "spawnRadius"},
	{"commandblockoutput", "commandBlockOutput"},
	{"sendcommandfeedback", "sendCommandFeedback"},
	{"showdeathmessages", "showDeathMessages"},
	{"maxcommandchainlength", "maxCommandChainLength"},
	{"falldamage", "fallDamage"},
	{"firedamage", "fireDamage"},
	{"drowningdamage", "drowningDamage"},
	{"freezedamage", "freezeDamage"},
	{"tntexplosiondropdecay", "tntExplosionDropDecay"},
}

func ConvertLevel(level cellerbedrock.Level, dataVersion int32) (nbt.Document, LevelReport, error) {
	// 1.21.4 将世界生成配置放在 Data.WorldGenSettings，不能沿用 26.x 的独立文件布局。
	source := level.NBT.Value
	report := LevelReport{}
	data := nbt.Compound{
		"DataVersion": dataVersion, "version": int32(19133),
		"Version":     nbt.Compound{"Id": dataVersion, "Name": "1.21.4", "Series": "main", "Snapshot": int8(0)},
		"initialized": int8(1),
		"DataPacks":   nbt.Compound{"Enabled": []string{"vanilla"}, "Disabled": []string{}},
	}
	data["LevelName"] = levelValue(source, "LevelName", "New World", &report)
	data["SpawnX"] = levelValue(source, "SpawnX", int32(0), &report)
	data["SpawnY"] = levelValue(source, "SpawnY", int32(64), &report)
	data["SpawnZ"] = levelValue(source, "SpawnZ", int32(0), &report)
	data["SpawnAngle"] = levelValue(source, "SpawnRotation", float32(0), &report)
	seed := levelValue(source, "RandomSeed", int64(0), &report)
	data["GameType"] = levelValue(source, "GameType", int32(0), &report)
	// Bedrock 的 Difficulty 是 Int，Java 用 Byte；Time 和 currentTick 分别对应昼夜与总时间。
	data["Difficulty"] = int8(levelValue(source, "Difficulty", int32(2), &report))
	dayTime := levelValue(source, "Time", int64(0), &report)
	gameTime := levelValue(source, "currentTick", dayTime, &report)
	data["Time"], data["DayTime"] = gameTime, dayTime
	data["allowCommands"] = levelValue(source, "commandsAllowed", int8(0), &report)
	data["hardcore"] = flag(source["IsHardcore"])
	// Bedrock 的 LastPlayed 是秒，Java 是毫秒。
	data["LastPlayed"] = levelValue(source, "LastPlayed", int64(0), &report) * 1000
	data["raining"] = ratio(source["rainLevel"])
	data["thundering"] = ratio(source["lightningLevel"])
	data["rainTime"] = levelValue(source, "rainTime", int32(0), &report)
	data["thunderTime"] = levelValue(source, "lightningTime", int32(0), &report)
	data["clearWeatherTime"] = int32(0)
	data["GameRules"] = convertGameRules(source)

	overworld := noiseDimension("overworld", "overworld", "overworld")
	if flat, ok := flatGenerator(source); ok {
		overworld["generator"] = flat
	}
	generate := int8(1)
	if value, ok := numberOf(source["GenerateStructures"]); ok {
		generate = flag(value)
	}
	data["WorldGenSettings"] = nbt.Compound{
		"seed": seed, "generate_features": generate, "bonus_chest": int8(0),
		"dimensions": nbt.Compound{
			"minecraft:overworld":  overworld,
			"minecraft:the_nether": noiseDimension("the_nether", "nether", "nether"),
			"minecraft:the_end":    noiseDimension("the_end", "end", ""),
		},
	}
	return nbt.Document{Value: nbt.Compound{"Data": data}}, report, nil
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
	for _, rule := range gameRules {
		value, exists := source[rule.bedrock]
		if !exists {
			continue
		}
		switch value := value.(type) {
		case int8:
			if value != 0 {
				rules[rule.java] = "true"
			} else {
				rules[rule.java] = "false"
			}
		case int32:
			rules[rule.java] = value
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
	layers := make([]nbt.Compound, 0, len(flat.BlockLayers))
	for _, layer := range flat.BlockLayers {
		name := namespaced(layer.BlockName)
		if state, ok := bedrock.ConvertState(ir.IRBlock(name + " []")); ok {
			name = state.NameWithNameSpace()
		}
		layers = append(layers, nbt.Compound{"block": name, "height": layer.Count})
	}
	biome := "minecraft:plains"
	if java, ok := tables.Biome(flat.BiomeID); ok {
		biome = java
	}
	return nbt.Compound{
		"type": "minecraft:flat",
		"settings": nbt.Compound{
			"biome": biome, "features": int8(0), "lakes": int8(0),
			"layers": layers, "structure_overrides": []string{},
		},
	}, true
}

func namespaced(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] == ':' {
			return name
		}
	}
	return "minecraft:" + name
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
