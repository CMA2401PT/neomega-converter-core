// Package brewing_stand 把 Java 酿造台转回 Bedrock，槽位顺序与正向相反。
package brewing_stand

import (
	"neomega-converter-core/java/blockentities"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:brewing_stand"

// Java 0-2 药水、3 原料、4 燃料 → Bedrock 0 原料、1-3 药水、4 燃料。
var bedrockSlot = map[int8]int8{0: 1, 1: 2, 2: 3, 3: 0, 4: 4}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	list, err := blockentities.Items(input)
	if err != nil {
		return blockentities.Result{}, err
	}
	for _, item := range list {
		if slot, ok := item["Slot"].(int8); ok {
			if mapped, ok := bedrockSlot[slot]; ok {
				item["Slot"] = mapped
			}
		}
	}
	entity := nbt.Compound{"id": "BrewingStand", "Items": list}
	if fuel, ok := blockentities.Number(input.Source["Fuel"]); ok {
		entity["FuelAmount"] = int8(fuel)
	}
	if brew, ok := blockentities.Number(input.Source["BrewTime"]); ok {
		entity["CookTime"] = int32(brew)
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
