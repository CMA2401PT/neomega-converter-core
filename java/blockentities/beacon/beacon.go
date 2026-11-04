// Package beacon 把 Java 信标效果转回 Bedrock 编号。
package beacon

import (
	"neomega-converter-core/java/blockentities"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:beacon"

var bedrockEffect = map[string]int32{
	"minecraft:speed": 1, "minecraft:haste": 3, "minecraft:strength": 5,
	"minecraft:jump_boost": 8, "minecraft:regeneration": 10, "minecraft:resistance": 11,
}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "Beacon"}
	for _, pair := range [][2]string{
		{"primary_effect", "primary"}, {"secondary_effect", "secondary"},
	} {
		name, ok := input.Source[pair[0]].(string)
		if !ok {
			continue
		}
		if value, ok := bedrockEffect[name]; ok {
			entity[pair[1]] = value
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
