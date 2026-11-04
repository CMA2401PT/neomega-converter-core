// Package actors 把 Bedrock 普通实体转成 Java 实体。
//
// 只转换两版含义一致的通用字段和少数特例（掉落物、船）；村民交易、乘骑、
// 宠物主人、盔甲架姿势等类型专属数据不在这里转换。
package actors

import (
	"strings"

	"neomega-converter-core/bedrock/internal/textcomponent"
	"neomega-converter-core/bedrock/items"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

// Bedrock 与 Java 名称不同的实体；其余同名。
var javaIDs = map[string]string{
	"zombie_pigman":       "zombified_piglin",
	"evocation_illager":   "evoker",
	"villager_v2":         "villager",
	"zombie_villager_v2":  "zombie_villager",
	"tropicalfish":        "tropical_fish",
	"ender_crystal":       "end_crystal",
	"xp_orb":              "experience_orb",
	"fireworks_rocket":    "firework_rocket",
	"eye_of_ender_signal": "eye_of_ender",
	"thrown_trident":      "trident",
	"xp_bottle":           "experience_bottle",
}

// 瞬时实体不转换：它们要么没有 Java 对应物，要么没有保留价值。
var skipped = map[string]bool{
	"arrow": true, "spectral_arrow": true, "trident": true, "snowball": true, "egg": true,
	"ender_pearl": true, "potion": true, "splash_potion": true, "lingering_potion": true,
	"fireball": true, "small_fireball": true, "dragon_fireball": true, "wither_skull": true,
	"shulker_bullet": true, "llama_spit": true, "fishing_bobber": true, "firework_rocket": true,
	"eye_of_ender": true, "experience_bottle": true, "area_effect_cloud": true, "lightning_bolt": true,
	"marker": true, "block_display": true, "item_display": true, "text_display": true, "interaction": true,
	"wind_charge": true, "breeze_wind_charge": true, "evoker_fangs": true, "player": true,
	"fishing_hook": true, "xp_bottle": true, "thrown_trident": true, "fireworks_rocket": true,
	"eye_of_ender_signal": true, "wither_skull_dangerous": true, "tripod_camera": true, "agent": true, "npc": true,
}

var boatWoods = []string{"oak", "spruce", "birch", "jungle", "acacia", "dark_oak", "mangrove", "bamboo", "cherry", "pale_oak"}

// Convert 返回 Java 实体。known 表示这是否是已知的 Bedrock 实体：
// 已知但被跳过或数据不完整时返回 nil, true；未知标识返回 nil, false。
func Convert(source nbt.Compound) (entity nbt.Compound, known bool) {
	identifier, _ := source["identifier"].(string)
	short := strings.TrimPrefix(identifier, "minecraft:")
	if short == "" {
		return nil, false
	}
	if skipped[short] {
		return nil, true
	}
	javaID, ok := javaIDs[short]
	if !ok {
		if !tables.KnownEntity(short) {
			return nil, false
		}
		javaID = short
	}
	if short == "boat" || short == "chest_boat" {
		javaID = boatID(source, short == "chest_boat")
	}
	position, ok := vector(source["Pos"], 3)
	if !ok {
		return nil, true
	}
	rotation, ok := vector(source["Rotation"], 2)
	if !ok {
		rotation = []float64{0, 0}
	}
	motion, ok := vector(source["Motion"], 3)
	if !ok {
		motion = []float64{0, 0, 0}
	}

	entity = nbt.Compound{
		"id":       "minecraft:" + javaID,
		"Pos":      position,
		"Rotation": floatList(rotation),
		"Motion":   motion,
	}
	if unique, ok := number(source["UniqueID"]); ok {
		entity["UUID"] = nbt.Array([]int32{0, 0, int32(uint64(unique) >> 32), int32(unique)})
	}
	for _, key := range []string{"OnGround", "Invulnerable", "CustomNameVisible", "Silent", "NoGravity"} {
		if value, ok := source[key].(int8); ok {
			entity[key] = value
		}
	}
	if value, ok := number(source["Fire"]); ok {
		entity["Fire"] = int16(value)
	}
	if value, ok := number(source["Air"]); ok {
		entity["Air"] = int16(value)
	}
	if value, ok := real(source["FallDistance"]); ok {
		entity["fall_distance"] = value
	}
	if name, ok := source["CustomName"].(string); ok && name != "" {
		entity["CustomName"] = textcomponent.Styled(name)
	}
	if tags, ok := source["Tags"].([]string); ok && len(tags) > 0 {
		entity["Tags"] = tags
	}
	if persistent, ok := source["Persistent"].(int8); ok && persistent != 0 {
		entity["PersistenceRequired"] = int8(1)
	}
	if baby, ok := source["IsBaby"].(int8); ok && baby != 0 {
		entity["Age"] = int32(-24000)
	}
	if health, ok := healthOf(source); ok {
		entity["Health"] = health
	}
	if equipment, ok := equipmentOf(source); ok {
		entity["equipment"] = equipment
	}

	switch short {
	case "item":
		if raw, ok := source["Item"].(nbt.Compound); ok {
			if converted := itemStack(raw); converted != nil {
				entity["Item"] = converted
			}
		}
		if age, ok := number(source["Age"]); ok {
			entity["Age"] = int16(age)
		}
		entity["PickupDelay"] = int16(0)
	case "falling_block":
		if block, ok := source["FallingBlock"].(nbt.Compound); ok {
			if state, ok := blockState(block); ok {
				entity["BlockState"] = state
			}
		}
	}
	return entity, true
}

func boatID(source nbt.Compound, chest bool) string {
	wood := "oak"
	if variant, ok := number(source["Variant"]); ok && variant >= 0 && int(variant) < len(boatWoods) {
		wood = boatWoods[variant]
	}
	kind := "_boat"
	if wood == "bamboo" {
		kind = "_raft"
	}
	if chest {
		kind = "_chest" + kind
	}
	return wood + kind
}

func blockState(source nbt.Compound) (nbt.Compound, bool) {
	name, ok := source["name"].(string)
	if !ok || name == "" {
		return nil, false
	}
	state := nbt.Compound{"Name": name}
	if properties, ok := source["states"].(nbt.Compound); ok {
		// Java 的 Properties 只接受字符串值。
		converted := nbt.Compound{}
		for key, value := range properties {
			if text, ok := value.(string); ok {
				converted[key] = text
			}
		}
		if len(converted) > 0 {
			state["Properties"] = converted
		}
	}
	return state, true
}

func itemStack(source nbt.Compound) nbt.Compound {
	converted := items.ConvertItem(source)
	if converted == nil {
		return nil
	}
	delete(converted, "Slot")
	return converted
}

type equipmentPair struct {
	java  string
	value any
}

func equipmentOf(source nbt.Compound) (nbt.Compound, bool) {
	pairs := []equipmentPair{
		{"mainhand", source["Mainhand"]},
		{"offhand", source["Offhand"]},
	}
	if armor, ok := source["Armor"].(nbt.Compound); ok {
		pairs = append(pairs,
			equipmentPair{"head", armor["Head"]},
			equipmentPair{"chest", armor["Chest"]},
			equipmentPair{"legs", armor["Legs"]},
			equipmentPair{"feet", armor["Feet"]},
		)
	}
	equipment := nbt.Compound{}
	for _, pair := range pairs {
		item, ok := pair.value.(nbt.Compound)
		if !ok {
			continue
		}
		if converted := itemStack(item); converted != nil {
			equipment[pair.java] = converted
		}
	}
	return equipment, len(equipment) > 0
}

func healthOf(source nbt.Compound) (float32, bool) {
	attributes, ok := nbt.Compounds(source["Attributes"])
	if !ok {
		return 0, false
	}
	for _, attribute := range attributes {
		if attribute["Name"] != "minecraft:health" {
			continue
		}
		if value, ok := real(attribute["Current"]); ok {
			return float32(value), true
		}
	}
	return 0, false
}

func vector(value any, size int) ([]float64, bool) {
	switch list := value.(type) {
	case []float32:
		if len(list) != size {
			return nil, false
		}
		out := make([]float64, size)
		for i, v := range list {
			out[i] = float64(v)
		}
		return out, true
	case []float64:
		if len(list) != size {
			return nil, false
		}
		return list, true
	default:
		return nil, false
	}
}

func floatList(values []float64) []float32 {
	out := make([]float32, len(values))
	for i, value := range values {
		out[i] = float32(value)
	}
	return out
}

func number(value any) (int64, bool) {
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

func real(value any) (float64, bool) {
	switch value := value.(type) {
	case float32:
		return float64(value), true
	case float64:
		return value, true
	default:
		return 0, false
	}
}
