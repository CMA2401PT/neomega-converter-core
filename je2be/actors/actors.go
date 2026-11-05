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

// Actor 是一条转换后的实体，ID 是 digp 里的 8 字节键。
type Actor struct {
	ID  [8]byte
	NBT nbt.Compound
}

// IsItemFrame 报告实体是否应转成 Bedrock 的方块 + 方块实体。
func IsItemFrame(source nbt.Compound) bool {
	id, _ := source["id"].(string)
	short := strings.TrimPrefix(id, "minecraft:")
	return short == "item_frame" || short == "glow_item_frame"
}

// Convert 返回第一条实体，主要用于只需要单个结果的调用方和测试。
func Convert(source nbt.Compound, report func(item string)) (nbt.Compound, [8]byte, bool) {
	actors, known := ConvertAll(source, report)
	if len(actors) == 0 {
		return nil, [8]byte{}, known
	}
	return actors[0].NBT, actors[0].ID, true
}

// ConvertAll 转换实体本身和 Passengers；known 表示父实体是否被识别。
func ConvertAll(source nbt.Compound, report func(item string)) ([]Actor, bool) {
	entity, id, known := convertOne(source, report)
	if entity == nil {
		return nil, known
	}
	out := []Actor{{ID: id, NBT: entity}}
	if passengers, ok := nbt.Compounds(source["Passengers"]); ok {
		for _, passenger := range passengers {
			children, childKnown := ConvertAll(passenger, report)
			if childKnown {
				out = append(out, children...)
			}
		}
	}
	return out, true
}

func convertOne(source nbt.Compound, report func(item string)) (nbt.Compound, [8]byte, bool) {
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
	chestItems := false
	if isBoat {
		bedrockID = boatKind
		chestItems = strings.Contains(short, "_chest_")
	} else if short == "chest_minecart" || short == "hopper_minecart" {
		chestItems = true
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
		"identifier":  "minecraft:" + bedrockID,
		"definitions": []string{"+minecraft:" + bedrockID},
		"Pos":         pos,
		"Rotation":    rotation,
		"Motion":      motion,
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
	persistent := false
	if value, ok := source["PersistenceRequired"].(int8); ok && value != 0 {
		persistent = true
	}
	if _, named := entity["CustomName"]; named || noDespawn[bedrockID] {
		persistent = true
	}
	entity["Persistent"] = boolByte(persistent)
	if short != "item" {
		if value, ok := source["Age"].(int32); ok && value < 0 {
			entity["IsBaby"] = int8(1)
		}
	}
	if value, ok := source["IsBaby"].(int8); ok && value != 0 {
		entity["IsBaby"] = int8(1)
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
	case short == "sheep":
		if color, ok := number(source["Color"]); ok {
			entity["Color"] = int8(color)
		}
		if sheared, ok := source["Sheared"].(int8); ok {
			entity["Sheared"] = sheared
		}
	case short == "horse":
		if variant, ok := number(source["Variant"]); ok {
			color := int(variant & 0xFF)
			marks := int((variant >> 8) & 0xFF)
			entity["Variant"] = int32(color)
			entity["MarkVariant"] = int32(marks)
			definitions, _ := entity["definitions"].([]string)
			if color < len(horseColors) {
				definitions = append(definitions, "+minecraft:base_"+horseColors[color])
			}
			if marks < len(horseMarks) {
				definitions = append(definitions, "+minecraft:markings_"+horseMarks[marks])
			}
			entity["definitions"] = definitions
		}
	case short == "painting":
		motif, _ := source["variant"].(string)
		motif = strings.TrimPrefix(motif, "minecraft:")
		entity["Motif"] = pascalCase(motif)
		facing := int32(0)
		if value, ok := number(source["facing"]); ok {
			facing = int32(value)
		}
		entity["Direction"] = int8(facing)
		width, height := paintingSize(motif)
		entity["block_pos"] = paintingBlockPos(pos, int(facing), width, height)
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
	if chestItems {
		if list, ok := chestItemList(source, report); ok {
			entity["ChestItems"] = list
		}
		if loot, ok := source["LootTable"].(string); ok && loot != "" {
			entity["LootTable"] = mapping.LootTableToBedrock(loot)
			if seed, ok := number(source["LootTableSeed"]); ok {
				entity["LootTableSeed"] = int32(seed)
			}
		}
	}

	return entity, actorID(source, short, pos), true
}

var horseColors = []string{"white", "creamy", "chestnut", "brown", "black", "gray", "darkbrown"}
var horseMarks = []string{"none", "white_details", "white_fields", "white_dots", "black_dots"}

func chestItemList(source nbt.Compound, report func(string)) ([]nbt.Compound, bool) {
	list, ok := nbt.Compounds(source["Items"])
	if !ok {
		return nil, false
	}
	out := make([]nbt.Compound, 0, len(list))
	for _, item := range list {
		if converted := items.ConvertItem(item, report); converted != nil {
			out = append(out, converted)
		}
	}
	return out, true
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

// Bedrock 里这些实体不会自然消失。
var noDespawn = map[string]bool{
	"villager_v2": true, "iron_golem": true, "snow_golem": true, "armor_stand": true,
	"boat": true, "chest_boat": true, "minecart": true, "chest_minecart": true,
	"hopper_minecart": true, "tnt_minecart": true, "command_block_minecart": true,
	"painting": true, "allay": true,
}

func boolByte(value bool) int8 {
	if value {
		return 1
	}
	return 0
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
