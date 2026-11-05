// Package lectern 把 Java 讲台转回 Bedrock。
package lectern

import (
	"neomega-converter-core/je2be/blockentities"
	"neomega-converter-core/je2be/items"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:lectern"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "Lectern"}
	if raw, ok := input.Source["Book"].(nbt.Compound); ok {
		if converted := items.ConvertItem(raw, input.ReportItem); converted != nil {
			delete(converted, "Slot")
			entity["book"] = converted
			entity["hasBook"] = int8(1)
			if page, ok := blockentities.Number(input.Source["Page"]); ok {
				entity["page"] = int32(page)
			}
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
