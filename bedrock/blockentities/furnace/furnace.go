// Package furnace 转换熔炉、高炉和烟熏炉的方块实体。
package furnace

import (
	"neomega-converter-core/bedrock/blockentities"
	"neomega-converter-core/bedrock/blockentities/container"
)

// Bedrock 的三种熔炉共用同一套计时字段，Java 方块实体 id 随方块变化。
const (
	ID       = "Furnace"
	BlastID  = "BlastFurnace"
	SmokerID = "Smoker"
)

func Convert(input blockentities.Input) (blockentities.Result, error) {
	javaID := "minecraft:furnace"
	switch input.Base.ShortName() {
	case "blast_furnace":
		javaID = "minecraft:blast_furnace"
	case "smoker":
		javaID = "minecraft:smoker"
	}
	entity, err := container.Common(input, javaID)
	if err != nil {
		return blockentities.Result{}, err
	}
	// Java 1.21.4 这四个字段都是 Short；Bedrock 可能用不同宽度。
	burn, _ := container.Number(input.Source["BurnTime"])
	cook, _ := container.Number(input.Source["CookTime"])
	duration, _ := container.Number(input.Source["BurnDuration"])
	entity["lit_time_remaining"] = int16(burn)
	entity["cooking_time_spent"] = int16(cook)
	entity["lit_total_time"] = int16(duration)
	entity["cooking_total_time"] = int16(200)
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
