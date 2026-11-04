// Package command_block 把 Java 命令方块转回 Bedrock。命令语言不翻译。
package command_block

import (
	"neomega-converter-core/java/blockentities"
	"neomega-converter-core/java/internal/textcomponent"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:command_block"

var copied = []string{"Command", "auto", "powered", "TrackOutput", "conditionMet"}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "CommandBlock"}
	for _, key := range copied {
		if value, ok := input.Source[key]; ok {
			entity[key] = value
		}
	}
	if value, ok := blockentities.Number(input.Source["SuccessCount"]); ok {
		entity["SuccessCount"] = int32(value)
	}
	if value, ok := input.Source["LastOutput"]; ok {
		entity["LastOutput"] = textcomponent.ToSection(value)
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
