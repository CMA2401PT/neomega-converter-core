package mob_spawner

import (
	"reflect"
	"testing"

	"neomega-converter-core/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base: "minecraft:mob_spawner []",
		Source: nbt.Compound{
			"Delay": int16(20), "SpawnCount": int32(4), "MaxNearbyEntities": int16(6),
			"EntityIdentifier": "minecraft:zombie",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "minecraft:mob_spawner" || entity["Delay"] != int16(20) ||
		entity["SpawnCount"] != int16(4) || entity["MaxNearbyEntities"] != int16(6) {
		t.Fatalf("fields: %#v", entity)
	}
	want := nbt.Compound{"entity": nbt.Compound{"id": "minecraft:zombie"}}
	if !reflect.DeepEqual(entity["SpawnData"], want) {
		t.Fatalf("SpawnData: %#v", entity["SpawnData"])
	}
	if list, ok := entity["SpawnPotentials"].([]nbt.Compound); !ok || len(list) != 0 {
		t.Fatalf("SpawnPotentials: %#v", entity["SpawnPotentials"])
	}
}
