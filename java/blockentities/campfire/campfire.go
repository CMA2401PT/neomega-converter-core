// Package campfire 把 Java 营火的四个槽位转回 Bedrock。
package campfire

import (
	"strconv"

	"neomega-converter-core/java/blockentities"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:campfire"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "Campfire"}
	list, err := blockentities.Items(input)
	if err != nil {
		return blockentities.Result{}, err
	}
	for _, item := range list {
		slot, ok := item["Slot"].(int8)
		if !ok || slot < 0 || slot > 3 {
			continue
		}
		delete(item, "Slot")
		suffix := strconv.Itoa(int(slot) + 1)
		entity["Item"+suffix] = item
	}
	if times, ok := intArray(input.Source["CookingTimes"]); ok {
		for index := 0; index < 4 && index < len(times); index++ {
			entity["ItemTime"+strconv.Itoa(index+1)] = times[index]
		}
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
