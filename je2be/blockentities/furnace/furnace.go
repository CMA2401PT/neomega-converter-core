// Package furnace 把 Java 熔炉、高炉和烟熏炉转回 Bedrock。
package furnace

import (
	"neomega-converter-core/je2be/blockentities"
)

const (
	ID       = "minecraft:furnace"
	BlastID  = "minecraft:blast_furnace"
	SmokerID = "minecraft:smoker"
)

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity, err := blockentities.Common(input)
	if err != nil {
		return blockentities.Result{}, err
	}
	id := "Furnace"
	switch input.SourceState.ShortName() {
	case "blast_furnace":
		id = "BlastFurnace"
	case "smoker":
		id = "Smoker"
	}
	entity["id"] = id
	// Java 的四个 Short 计时字段，Bedrock 只有三个。
	for java, bedrock := range map[string]string{
		"lit_time_remaining": "BurnTime",
		"cooking_time_spent": "CookTime",
		"lit_total_time":     "BurnDuration",
	} {
		if value, ok := blockentities.Number(input.Source[java]); ok {
			entity[bedrock] = int16(value)
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
