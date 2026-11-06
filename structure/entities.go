package structure

import (
	"maps"
	"strings"

	"celler"
	cellerjava "celler/java"
	nbt "neomega-nbt/presets/v1"
)

func javaBlockEntity(x, y, z int32, id string, data nbt.Compound) cellerjava.BlockEntity {
	// 方块实体跟着方块落到世界坐标。Data 里的原字段保留，坐标和 id 按 Java 区块重写。
	value := nbt.Compound{}
	if data != nil {
		value = maps.Clone(data)
	}
	value["id"] = namespaced(id)
	value["x"], value["y"], value["z"] = x, y, z
	return cellerjava.BlockEntity{Pos: celler.BlockPos{X: x, Y: y, Z: z}, NBT: value}
}

func namespaced(id string) string {
	if id == "" || strings.Contains(id, ":") {
		return id
	}
	return "minecraft:" + id
}

// schemEntity 读取 Sponge 的 Id/Pos/Data。Pos 相对结构最小角，调用方已把最小角放在原点。
// 一条实体缺字段就跳过，不让它挡住其余方块。
func schemEntity(raw nbt.Compound) (nbt.Compound, bool) {
	id, _ := raw["Id"].(string)
	if id == "" {
		id, _ = raw["id"].(string)
	}
	pos, ok := float3(raw["Pos"])
	if id == "" || !ok {
		return nil, false
	}
	out := nbt.Compound{}
	if data, ok := raw["Data"].(map[string]any); ok {
		out = maps.Clone(data)
	} else {
		out = maps.Clone(raw)
		delete(out, "Id")
		delete(out, "Data")
	}
	out["id"] = namespaced(id)
	out["Pos"] = pos
	return out, true
}

// shiftEntity 把投影里的 Pos 加上区域的原始角。负尺寸时这个角不是最小角。
func shiftEntity(raw nbt.Compound, origin [3]int32) (nbt.Compound, bool) {
	id, _ := raw["id"].(string)
	pos, ok := float3(raw["Pos"])
	if id == "" || !ok {
		return nil, false
	}
	out := maps.Clone(raw)
	for axis := range 3 {
		pos[axis] += float64(origin[axis])
	}
	out["id"] = namespaced(id)
	out["Pos"] = pos
	return out, true
}

func float3(value any) ([]float64, bool) {
	switch typed := value.(type) {
	case []float64:
		if len(typed) != 3 {
			return nil, false
		}
		return append([]float64(nil), typed...), true
	case []float32:
		if len(typed) != 3 {
			return nil, false
		}
		return []float64{float64(typed[0]), float64(typed[1]), float64(typed[2])}, true
	default:
		return nil, false
	}
}
