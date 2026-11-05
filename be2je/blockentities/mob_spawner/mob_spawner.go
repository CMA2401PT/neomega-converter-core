// Package mob_spawner 转换刷怪笼方块实体。
package mob_spawner

import (
	"neomega-converter-core/be2je/blockentities"
	"neomega-converter-core/be2je/blockentities/container"
	nbt "neomega-nbt/presets/v1"
)

const ID = "MobSpawner"

// 两版同名的七个计时字段。Java 1.21.4 用 Short。
var timers = []string{
	"Delay", "MinSpawnDelay", "MaxSpawnDelay", "SpawnCount",
	"MaxNearbyEntities", "RequiredPlayerRange", "SpawnRange",
}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{
		"id":              "minecraft:mob_spawner",
		"SpawnPotentials": []nbt.Compound{},
	}
	for _, key := range timers {
		if value, ok := container.Number(input.Source[key]); ok {
			entity[key] = int16(value)
		}
	}
	// Bedrock 只保存一个 EntityIdentifier，Java 需要嵌在 SpawnData.entity 里。
	if id, ok := input.Source["EntityIdentifier"].(string); ok && id != "" {
		entity["SpawnData"] = nbt.Compound{"entity": nbt.Compound{"id": id}}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
