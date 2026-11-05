// Package shulker_box 把 Java 潜影盒转回 Bedrock。
package shulker_box

import (
	"neomega-converter-core/je2be/blockentities"
)

const ID = "minecraft:shulker_box"

var bedrockFacing = map[string]int8{"down": 0, "up": 1, "north": 2, "south": 3, "west": 4, "east": 5}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity, err := blockentities.Common(input)
	if err != nil {
		return blockentities.Result{}, err
	}
	entity["id"] = "ShulkerBox"
	if facing, ok := blockentities.StateString(input.SourceState, "facing"); ok {
		if value, ok := bedrockFacing[facing]; ok {
			entity["facing"] = value
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
