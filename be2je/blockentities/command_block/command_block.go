// Package command_block 转换命令方块方块实体。命令语言本身不翻译。
package command_block

import (
	"neomega-converter-core/be2je/blockentities"
	"neomega-converter-core/be2je/blockentities/container"
	"neomega-converter-core/be2je/internal/textcomponent"
	nbt "neomega-nbt/presets/v1"
)

const ID = "CommandBlock"

// 两版同名的字段，类型也一致的直接复制。
var copied = []string{"Command", "auto", "powered", "TrackOutput", "conditionMet"}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "minecraft:command_block"}
	for _, key := range copied {
		if value, ok := input.Source[key]; ok {
			entity[key] = value
		}
	}
	if value, ok := container.Number(input.Source["SuccessCount"]); ok {
		entity["SuccessCount"] = int32(value)
	}
	// Java 1.21.4 的 LastOutput 是文本组件，不能保留原文的 § 格式码。
	if value, ok := input.Source["LastOutput"].(string); ok {
		entity["LastOutput"] = textcomponent.Styled(value)
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
