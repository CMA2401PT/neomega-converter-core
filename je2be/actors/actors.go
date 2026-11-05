// Package actors 把 Java 普通实体转成 Bedrock 的 actorprefix NBT。
// 只处理两版含义一致的通用字段和掉落物、船、下落方块；类型专属数据不在这里转换。
package actors

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"slices"
	"strings"

	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter-core/je2be/internal/textcomponent"
	"neomega-converter-core/je2be/items"
	"neomega-converter-core/mapping"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

// Convert 返回 Bedrock actor NBT 和 digp 键。known 表示实体是否被识别：
// 已知但跳过时返回 nil, true；未知标识返回 nil, false。
func Convert(source nbt.Compound, report func(item string)) (nbt.Compound, [8]byte, bool) {
	identifier, _ := source["id"].(string)
	short := strings.TrimPrefix(identifier, "minecraft:")
	if short == "" {
		return nil, [8]byte{}, false
	}
	if mapping.SkipEntity(short) {
		return nil, [8]byte{}, true
	}
	bedrockID := mapping.BedrockEntityID(short)
	boatKind, boatVariant, isBoat := boat(short)
	if isBoat {
		bedrockID = boatKind
	} else if !tables.KnownEntity(bedrockID) {
		return nil, [8]byte{}, false
	}

	pos, ok := floats(source["Pos"])
	if !ok {
		return nil, [8]byte{}, true
	}
	rotation, ok := floats(source["Rotation"])
	if !ok {
		rotation = []float32{0, 0}
	}
	motion, ok := floats(source["Motion"])
	if !ok {
		motion = []float32{0, 0, 0}
	}
	entity := nbt.Compound{
		"identifier": "minecraft:" + bedrockID,
		"Pos":        pos,
		"Rotation":   rotation,
		"Motion":     motion,
	}
	if uuid, ok := uuidInts(source["UUID"]); ok {
		entity["UniqueID"] = int64(uint64(uint32(uuid[2]))<<32 | uint64(uint32(uuid[3])))
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
	if value, ok := real(source["fall_distance"]); ok {
		entity["FallDistance"] = value
	}
	if name, ok := source["CustomName"].(string); ok && name != "" {
		entity["CustomName"] = textcomponent.ToSection(name)
	}
	if tags, ok := source["Tags"].([]string); ok && len(tags) > 0 {
		entity["Tags"] = tags
	}
	if value, ok := source["PersistenceRequired"].(int8); ok && value != 0 {
		entity["Persistent"] = int8(1)
	}
	if value, ok := source["Age"].(int32); ok && value == -24000 {
		entity["IsBaby"] = int8(1)
	}
	if health, ok := real(source["Health"]); ok {
		entity["Attributes"] = []nbt.Compound{{"Name": "minecraft:health", "Current": float32(health)}}
	}
	if equipment, ok := equipmentOf(source, report); ok {
		for key, value := range equipment {
			entity[key] = value
		}
	}

	switch {
	case isBoat:
		entity["identifier"] = "minecraft:" + boatKind
		entity["Variant"] = boatVariant
	case short == "item":
		if raw, ok := source["Item"].(nbt.Compound); ok {
			if converted := items.ConvertItem(raw, report); converted != nil {
				delete(converted, "Slot")
				entity["Item"] = converted
			}
		}
		if value, ok := number(source["Age"]); ok {
			entity["Age"] = int16(value)
		}
	case short == "falling_block":
		if raw, ok := source["BlockState"].(nbt.Compound); ok {
			if name, ok := raw["Name"].(string); ok {
				java := ir.IRBlock(name + " []")
				if properties, ok := raw["Properties"].(nbt.Compound); ok && len(properties) > 0 {
					java, _ = ir.NewIRBlockFromGo(name, properties)
				}
				if state, ok := converts.ConvertFromJava(java); ok {
					entity["FallingBlock"] = nbt.Compound{
						"name": state.NameWithNameSpace(), "states": state.States().GoMap(),
					}
				}
			}
		}
	}

	return entity, actorID(source, short, pos), true
}

func equipmentOf(source nbt.Compound, report func(item string)) (nbt.Compound, bool) {
	equipment := javaEquipment(source)
	if equipment == nil {
		return nil, false
	}
	out := nbt.Compound{}
	// Bedrock 的 Armor 固定是四件物品的 list，顺序 head、chest、legs、feet；
	// 没有装备的槽位也要放空物品，不能省略。
	armor := make([]nbt.Compound, 0, 4)
	hasArmor := false
	for _, slot := range []string{"head", "chest", "legs", "feet"} {
		item, ok := equipment[slot].(nbt.Compound)
		if !ok {
			armor = append(armor, emptyEquipmentItem())
			continue
		}
		converted := items.ConvertItem(item, report)
		if converted == nil {
			armor = append(armor, emptyEquipmentItem())
			continue
		}
		delete(converted, "Slot")
		armor = append(armor, converted)
		hasArmor = true
	}
	if hasArmor {
		out["Armor"] = armor
	}
	for _, slot := range []string{"mainhand", "offhand"} {
		item, ok := equipment[slot].(nbt.Compound)
		if !ok {
			continue
		}
		converted := items.ConvertItem(item, report)
		if converted == nil {
			continue
		}
		delete(converted, "Slot")
		if slot == "mainhand" {
			out["Mainhand"] = []nbt.Compound{converted}
		} else {
			out["Offhand"] = []nbt.Compound{converted}
		}
	}
	return out, len(out) > 0
}

// javaEquipment 统一 1.21.5+ 的 equipment 和旧版的 ArmorItems/HandItems。
// 返回 nil 表示源实体没有任何装备字段。
func javaEquipment(source nbt.Compound) nbt.Compound {
	if equipment, ok := source["equipment"].(nbt.Compound); ok {
		return equipment
	}
	out := nbt.Compound{}
	if list, ok := nbt.Compounds(source["ArmorItems"]); ok && len(list) == 4 {
		out["feet"], out["legs"], out["chest"], out["head"] = list[0], list[1], list[2], list[3]
	}
	if list, ok := nbt.Compounds(source["HandItems"]); ok && len(list) == 2 {
		out["mainhand"], out["offhand"] = list[0], list[1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func emptyEquipmentItem() nbt.Compound {
	return nbt.Compound{"Count": int8(0), "Damage": int16(0), "Name": "", "WasPickedUp": int8(0)}
}

func boat(id string) (string, int32, bool) {
	wood, kind, chest := "", "", false
	switch {
	case strings.HasSuffix(id, "_chest_raft"):
		wood, kind, chest = "bamboo", "boat", true
	case strings.HasSuffix(id, "_chest_boat"):
		wood, kind, chest = strings.TrimSuffix(id, "_chest_boat"), "boat", true
	case strings.HasSuffix(id, "_raft"):
		wood, kind = "bamboo", "boat"
	case strings.HasSuffix(id, "_boat"):
		wood, kind = strings.TrimSuffix(id, "_boat"), "boat"
	default:
		return "", 0, false
	}
	index := slices.Index(mapping.BoatWoods, wood)
	if index < 0 {
		return "", 0, false
	}
	if chest {
		kind = "chest_boat"
	}
	return kind, int32(index), true
}

func actorID(source nbt.Compound, short string, pos []float32) [8]byte {
	var out [8]byte
	if uuid, ok := uuidInts(source["UUID"]); ok {
		binary.LittleEndian.PutUint32(out[0:4], uint32(uuid[2]))
		binary.LittleEndian.PutUint32(out[4:8], uint32(uuid[3]))
		return out
	}
	hash := fnv.New64a()
	hash.Write([]byte(short))
	for _, value := range pos {
		var raw [4]byte
		binary.LittleEndian.PutUint32(raw[:], math.Float32bits(value))
		hash.Write(raw[:])
	}
	binary.LittleEndian.PutUint64(out[:], hash.Sum64())
	return out
}

func floats(value any) ([]float32, bool) {
	switch value := value.(type) {
	case []float32:
		return value, len(value) == 3 || len(value) == 2
	case []float64:
		out := make([]float32, len(value))
		for index, current := range value {
			out[index] = float32(current)
		}
		return out, len(value) == 3 || len(value) == 2
	default:
		return nil, false
	}
}

func uuidInts(value any) ([4]int32, bool) {
	if values, ok := nbt.ArrayValues[int32](value); ok && len(values) == 4 {
		return [4]int32{values[0], values[1], values[2], values[3]}, true
	}
	if values, ok := value.([]int32); ok && len(values) == 4 {
		return [4]int32{values[0], values[1], values[2], values[3]}, true
	}
	return [4]int32{}, false
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
