// Package brewing_stand 转换酿造台方块实体，两版槽位顺序不同。
package brewing_stand

import (
	"neomega-converter-core/bedrock/blockentities"
	"neomega-converter-core/bedrock/blockentities/container"
	nbt "neomega-nbt/presets/v1"
)

const ID = "BrewingStand"

// Bedrock：0 原料、1-3 药水、4 燃料；Java：0-2 药水、3 原料、4 燃料。
var javaSlot = map[int8]int8{0: 3, 1: 0, 2: 1, 3: 2, 4: 4}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity, err := container.Common(input, "minecraft:brewing_stand")
	if err != nil {
		return blockentities.Result{}, err
	}
	list, _ := entity["Items"].([]nbt.Compound)
	for _, item := range list {
		slot, ok := item["Slot"].(int8)
		if !ok {
			continue
		}
		if mapped, ok := javaSlot[slot]; ok {
			item["Slot"] = mapped
		}
	}
	fuel, _ := container.Number(input.Source["FuelAmount"])
	entity["Fuel"] = int8(fuel)
	brew, _ := container.Number(input.Source["CookTime"])
	entity["BrewTime"] = int16(brew)
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
