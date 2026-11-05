// Package beacon 转换信标方块实体。
package beacon

import (
	"neomega-converter-core/be2je/blockentities"
	"neomega-converter-core/be2je/blockentities/container"
	nbt "neomega-nbt/presets/v1"
)

const ID = "Beacon"

// 信标只允许这六种效果，编号来自 Bedrock 的效果 id。
var effects = map[int64]string{
	1: "speed", 3: "haste", 5: "strength", 8: "jump_boost", 10: "regeneration", 11: "resistance",
}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "minecraft:beacon"}
	for _, key := range []string{"primary", "secondary"} {
		value, ok := container.Number(input.Source[key])
		if !ok {
			continue
		}
		if name, ok := effects[value]; ok {
			entity[key+"_effect"] = "minecraft:" + name
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
