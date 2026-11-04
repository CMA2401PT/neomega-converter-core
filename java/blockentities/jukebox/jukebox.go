// Package jukebox 把 Java 唱片机转回 Bedrock。
package jukebox

import (
	"neomega-converter-core/java/blockentities"
	"neomega-converter-core/java/items"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:jukebox"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "Jukebox"}
	if raw, ok := input.Source["RecordItem"].(nbt.Compound); ok {
		if converted := items.ConvertItem(raw, input.ReportItem); converted != nil {
			delete(converted, "Slot")
			entity["RecordItem"] = converted
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
