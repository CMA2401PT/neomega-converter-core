package structure

import (
	"maps"
	"strings"

	"celler"
	cellerjava "celler/java"
	nbt "neomega-nbt/presets/v1"
)

// javaBlockEntity 把方块实体搬到 Java 世界坐标：保留其余字段，重写 id 和 x/y/z。
// data 是 Sponge 的 Data 内容或原版方块实体的字段表；坐标由调用方换算。
func javaBlockEntity(x, y, z int32, id string, data nbt.Compound) cellerjava.BlockEntity {
	value := nbt.Compound{}
	if data != nil {
		value = maps.Clone(data)
	}
	value["id"] = namespaced(id)
	value["x"], value["y"], value["z"] = x, y, z
	return cellerjava.BlockEntity{Pos: celler.BlockPos{X: x, Y: y, Z: z}, NBT: value}
}

// namespaced 给没有命名空间的 id 补 minecraft:；结构文件里两种写法都有。
func namespaced(id string) string {
	if id == "" || strings.Contains(id, ":") {
		return id
	}
	return "minecraft:" + id
}

// tilePosition 读取方块实体的坐标：Sponge 包装写在 Pos，原版写 x/y/z；Pos 优先。
func tilePosition(tile nbt.Compound) ([3]int32, bool) {
	if pos, ok := nbtInt3(tile["Pos"]); ok {
		return pos, true
	}
	x, okX := nbtInt32(tile["x"])
	y, okY := nbtInt32(tile["y"])
	z, okZ := nbtInt32(tile["z"])
	return [3]int32{x, y, z}, okX && okY && okZ
}

// spongePayload 取方块实体真正的字段表：Sponge 包装把内容放在 Data 下，原版平铺在同一层。
// 只有带 Pos 的条目才按包装解开，避免把普通条目里同名的 Data 当成内容。
func spongePayload(tile nbt.Compound) nbt.Compound {
	if _, wrapped := nbtInt3(tile["Pos"]); !wrapped {
		return tile
	}
	if data, ok := tile["Data"].(map[string]any); ok {
		return data
	}
	return tile
}

// schemEntity 读取 Sponge 的 Id/Pos/Data。Pos 相对结构最小角，调用方已把最小角放在原点。
// 一条实体缺字段就跳过，不让它挡住其余方块。
func schemEntity(raw nbt.Compound) (nbt.Compound, bool) {
	id := nbtID(raw)
	pos, ok := nbtFloat3(raw["Pos"])
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
	id := nbtID(raw)
	pos, ok := nbtFloat3(raw["Pos"])
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
