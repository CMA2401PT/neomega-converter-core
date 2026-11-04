// Package decorated_pot 转换饰纹罐方块实体。
package decorated_pot

import (
	"neomega-converter-core/bedrock/blockentities"
	"neomega-converter-core/bedrock/items"
	nbt "neomega-nbt/presets/v1"
)

const ID = "DecoratedPot"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "minecraft:decorated_pot"}
	if sherds, ok := input.Source["sherds"].([]string); ok && len(sherds) > 0 {
		entity["sherds"] = sherds
	}
	if raw, ok := input.Source["item"].(nbt.Compound); ok {
		if converted := items.ConvertItem(raw); converted != nil {
			delete(converted, "Slot")
			entity["item"] = converted
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
