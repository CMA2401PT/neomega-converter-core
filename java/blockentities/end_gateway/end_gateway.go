// Package end_gateway 把 Java 末地折跃门转回 Bedrock。
package end_gateway

import (
	"neomega-converter-core/java/blockentities"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:end_gateway"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "EndGateway"}
	if portal, ok := intArray(input.Source["exit_portal"]); ok && len(portal) == 3 {
		entity["ExitPortal"] = []int32{portal[0], portal[1], portal[2]}
	}
	if age, ok := blockentities.Number(input.Source["Age"]); ok {
		entity["Age"] = int32(age)
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}

func intArray(value any) ([]int32, bool) {
	if values, ok := nbt.ArrayValues[int32](value); ok {
		return values, true
	}
	if values, ok := value.([]int32); ok {
		return values, true
	}
	return nil, false
}
