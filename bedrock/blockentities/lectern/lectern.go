package lectern

import (
	"fmt"
	"strconv"

	"neomega-blocks/ir"
	"neomega-converter-core/bedrock/blockentities"
	"neomega-converter-core/bedrock/blockentities/container"
	"neomega-converter-core/bedrock/items"
	nbt "neomega-nbt/presets/v1"
)

const ID string = "Lectern"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// 空讲台可能省略 hasBook，因此缺失时使用零值。
	var hasBook int8
	if value, exists := input.Source["hasBook"]; exists {
		var ok bool
		hasBook, ok = value.(int8)
		if !ok {
			return blockentities.Result{}, fmt.Errorf("lectern hasBook must be a Byte")
		}
	}
	states := input.Base.States().Set("has_book", strconv.FormatBool(hasBook != 0))
	state := ir.IRBlock(input.Base.NameWithNameSpace() + " " + string(states)).Tidy()
	entity := nbt.Compound{"id": "minecraft:lectern"}
	if raw, ok := input.Source["book"].(nbt.Compound); ok {
		if converted := items.ConvertItem(raw); converted != nil {
			delete(converted, "Slot")
			entity["Book"] = converted
			if page, ok := container.Number(input.Source["page"]); ok {
				entity["Page"] = int32(page)
			}
		}
	}
	return blockentities.Result{State: state, BlockEntity: entity}, nil
}
