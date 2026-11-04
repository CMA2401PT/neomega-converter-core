// Package end_gateway 转换末地折跃门方块实体。
package end_gateway

import (
	"neomega-converter-core/bedrock/blockentities"
	"neomega-converter-core/bedrock/blockentities/container"
	nbt "neomega-nbt/presets/v1"
)

const ID = "EndGateway"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "minecraft:end_gateway"}
	if portal, ok := input.Source["ExitPortal"].([]int32); ok && len(portal) == 3 {
		// Bedrock 存 List<Int>，Java 要 IntArray。
		entity["exit_portal"] = nbt.Array(portal)
	}
	if age, ok := container.Number(input.Source["Age"]); ok {
		entity["Age"] = age
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
