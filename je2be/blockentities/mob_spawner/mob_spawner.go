// Package mob_spawner 把 Java 刷怪笼转回 Bedrock。
package mob_spawner

import (
	"neomega-converter-core/je2be/blockentities"
	"neomega-converter-core/mapping"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:mob_spawner"

var timers = []string{
	"Delay", "MinSpawnDelay", "MaxSpawnDelay", "SpawnCount",
	"MaxNearbyEntities", "RequiredPlayerRange", "SpawnRange",
}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "MobSpawner"}
	for _, key := range timers {
		if value, ok := blockentities.Number(input.Source[key]); ok {
			entity[key] = int16(value)
		}
	}
	if spawn, ok := input.Source["SpawnData"].(nbt.Compound); ok {
		if entityData, ok := spawn["entity"].(nbt.Compound); ok {
			if id, ok := entityData["id"].(string); ok && id != "" {
				entity["EntityIdentifier"] = "minecraft:" + mapping.BedrockEntityID(id)
			}
		}
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
