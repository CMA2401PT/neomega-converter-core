package je2be

import (
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter-core"
	javaitems "neomega-converter-core/je2be/items"
	nbt "neomega-nbt/presets/v1"
)

// placeItemFrame 把 Java 的展示框实体写回 Bedrock 的 frame 方块和方块实体。
// 返回 true 表示这条实体已处理（包括因位置不合法而跳过的情况）。
func placeItemFrame(result *converter.BedrockChunk, source nbt.Compound, report func(string)) bool {
	pos, ok := framePosition(source)
	if !ok {
		return true
	}
	if pos[0]>>4 != result.Pos.X || pos[2]>>4 != result.Pos.Z {
		return true
	}
	section := result.Sections[pos[1]>>4]
	if section == nil || len(section.Layers) == 0 || section.Layers[0] == nil {
		return true
	}
	local := layer_storage.At(pos[0], pos[1], pos[2])
	// 只替换空气；水下的展示框需要额外的水层，当前跳过，避免丢失水。
	if name := section.Layers[0].Get(local).State.NameWithNameSpace(); name != "minecraft:air" {
		return true
	}
	facing := int32(0)
	if value, ok := frameNumber(source["Facing"]); ok {
		facing = int32(value)
	}
	mapBit := int8(0)
	var item nbt.Compound
	if raw, ok := source["Item"].(nbt.Compound); ok {
		item = javaitems.ConvertItem(raw, report)
		if item != nil {
			delete(item, "Slot")
			if item["Name"] == "minecraft:filled_map" {
				mapBit = 1
			}
		}
	}
	name, entityID := "minecraft:frame", "ItemFrame"
	id, _ := source["id"].(string)
	if id == "minecraft:glow_item_frame" {
		name, entityID = "minecraft:glow_frame", "GlowItemFrame"
	}
	state, err := ir.NewIRBlockFromGo(name, map[string]any{
		"facing_direction":     facing,
		"item_frame_map_bit":   mapBit,
		"item_frame_photo_bit": int8(0),
	})
	if err != nil {
		return true
	}
	block := cellerbedrock.Block{State: state.Tidy(), Version: bedrockBlockVersion}
	section.Layers[0].Set(local, block)
	entity := nbt.Compound{
		"id": entityID, "isMovable": int8(1),
		"x": pos[0], "y": pos[1], "z": pos[2],
	}
	if item != nil {
		entity["Item"] = item
		rotation := int64(0)
		if value, ok := frameNumber(source["ItemRotation"]); ok {
			rotation = value
		}
		entity["ItemRotation"] = float32(rotation * 45)
		drop := 1.0
		if value, ok := frameReal(source["ItemDropChance"]); ok {
			drop = value
		}
		entity["ItemDropChance"] = float32(drop)
	}
	result.BlockEntities = append(result.BlockEntities, entity)
	return true
}

func framePosition(source nbt.Compound) ([3]int32, bool) {
	if values, ok := nbt.ArrayValues[int32](source["block_pos"]); ok && len(values) == 3 {
		return [3]int32{values[0], values[1], values[2]}, true
	}
	if values, ok := source["block_pos"].([]int32); ok && len(values) == 3 {
		return [3]int32{values[0], values[1], values[2]}, true
	}
	x, okX := frameNumber(source["TileX"])
	y, okY := frameNumber(source["TileY"])
	z, okZ := frameNumber(source["TileZ"])
	if !okX || !okY || !okZ {
		return [3]int32{}, false
	}
	return [3]int32{int32(x), int32(y), int32(z)}, true
}

func frameNumber(value any) (int64, bool) {
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

func frameReal(value any) (float64, bool) {
	switch value := value.(type) {
	case float32:
		return float64(value), true
	case float64:
		return value, true
	default:
		return 0, false
	}
}
