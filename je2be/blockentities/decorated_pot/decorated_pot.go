// Package decorated_pot 把 Java 饰纹罐转回 Bedrock。
package decorated_pot

import (
	"neomega-converter-core/je2be/blockentities"
	"neomega-converter-core/je2be/items"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:decorated_pot"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "DecoratedPot"}
	if sherds, ok := input.Source["sherds"].([]string); ok && len(sherds) > 0 {
		entity["sherds"] = sherds
	}
	if raw, ok := input.Source["item"].(nbt.Compound); ok {
		if converted := items.ConvertItem(raw, input.ReportItem); converted != nil {
			delete(converted, "Slot")
			entity["item"] = converted
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
