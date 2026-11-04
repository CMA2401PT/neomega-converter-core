// Package campfire 转换营火与灵魂营火的四个烹饪槽。
package campfire

import (
	"strconv"

	"neomega-converter-core/bedrock/blockentities"
	"neomega-converter-core/bedrock/blockentities/container"
	"neomega-converter-core/bedrock/items"
	nbt "neomega-nbt/presets/v1"
)

const ID = "Campfire"

// Java 营火的每个槽位烹饪总时长固定 600 tick。
const cookingTotal = int32(600)

func Convert(input blockentities.Input) (blockentities.Result, error) {
	javaID := "minecraft:campfire"
	if input.Base.ShortName() == "soul_campfire" {
		javaID = "minecraft:soul_campfire"
	}
	entity := nbt.Compound{"id": javaID}
	list := []nbt.Compound{}
	times := make([]int32, 4)
	for i := 1; i <= 4; i++ {
		suffix := strconv.Itoa(i)
		if raw, ok := input.Source["Item"+suffix].(nbt.Compound); ok {
			if converted := items.ConvertItem(raw); converted != nil {
				converted["Slot"] = int8(i - 1)
				list = append(list, converted)
			}
		}
		value, _ := container.Number(input.Source["ItemTime"+suffix])
		times[i-1] = int32(value)
	}
	totals := []int32{cookingTotal, cookingTotal, cookingTotal, cookingTotal}
	entity["Items"] = list
	entity["CookingTimes"] = nbt.Array(times)
	entity["CookingTotalTimes"] = nbt.Array(totals)
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
