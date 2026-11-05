// Package actors 把 Bedrock 普通实体转成 Java 实体。
//
// 只转换两版含义一致的通用字段和少数特例（掉落物、船）；村民交易、乘骑、
// 宠物主人、盔甲架姿势等类型专属数据不在这里转换。
package actors

import (
	"strings"

	"neomega-converter-core/be2je/internal/textcomponent"
	"neomega-converter-core/be2je/items"
	"neomega-converter-core/mapping"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

// Convert 返回 Java 实体。known 表示这是否是已知的 Bedrock 实体：
// 已知但被跳过或数据不完整时返回 nil, true；未知标识返回 nil, false。
func Convert(source nbt.Compound) (entity nbt.Compound, known bool) {
	return ConvertWithReport(source, nil)
}

// ConvertWithReport 与 Convert 相同，物品转不出时把物品名交给 report。
// report 可以为 nil。
func ConvertWithReport(source nbt.Compound, report func(item string)) (entity nbt.Compound, known bool) {
	identifier, _ := source["identifier"].(string)
	short := strings.TrimPrefix(identifier, "minecraft:")
	if short == "" {
		return nil, false
	}
	if mapping.SkipEntity(short) {
		return nil, true
	}
	javaID, ok := mapping.JavaEntityID(short)
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
	if equipment, ok := equipmentOf(source, report); ok {
		entity["equipment"] = equipment
	}

	switch short {
	case "item":
		if raw, ok := source["Item"].(nbt.Compound); ok {
			if converted := itemStack(raw, report); converted != nil {
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
	if variant, ok := number(source["Variant"]); ok && variant >= 0 && int(variant) < len(mapping.BoatWoods) {
		wood = mapping.BoatWoods[variant]
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

func itemStack(source nbt.Compound, report func(item string)) nbt.Compound {
	converted := items.ConvertItemReported(source, report)
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

func equipmentOf(source nbt.Compound, report func(item string)) (nbt.Compound, bool) {
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
		if converted := itemStack(item, report); converted != nil {
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
