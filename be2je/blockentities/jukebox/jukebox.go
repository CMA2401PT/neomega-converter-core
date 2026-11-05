package jukebox

import (
	"strconv"

	"neomega-blocks/ir"
	"neomega-converter-core/be2je/blockentities"
	"neomega-converter-core/be2je/items"
	nbt "neomega-nbt/presets/v1"
)

const ID string = "Jukebox"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// 唱片是否存在由物品 compound 表达，基础状态只有默认值。
	raw, hasRecord := input.Source["RecordItem"].(nbt.Compound)
	states := input.Base.States().Set("has_record", strconv.FormatBool(hasRecord))
	state := ir.IRBlock(input.Base.NameWithNameSpace() + " " + string(states)).Tidy()
	entity := nbt.Compound{"id": "minecraft:jukebox"}
	if hasRecord {
		if converted := items.ConvertItemReported(raw, input.ReportItem); converted != nil {
			delete(converted, "Slot")
			entity["RecordItem"] = converted
		}
	}
	return blockentities.Result{State: state, BlockEntity: entity}, nil
}
