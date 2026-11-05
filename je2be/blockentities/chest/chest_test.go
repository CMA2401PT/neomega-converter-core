package chest

import (
	"testing"

	"celler"
	"neomega-converter-core/je2be/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		SourceState: `minecraft:chest ["facing"="north","type"="left","waterlogged"="false"]`,
		Base:        `minecraft:chest []`,
		WorldPos:    celler.BlockPos{X: 10, Y: 64, Z: 20},
		Source: nbt.Compound{
			"id":            "minecraft:chest",
			"Items":         []nbt.Compound{{"id": "minecraft:oak_log", "count": int32(2), "Slot": int8(0)}},
			"CustomName":    `{"text":"x"}`,
			"LootTable":     "minecraft:chests/simple_dungeon",
			"LootTableSeed": int64(3),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "Chest" || entity["pairx"] != int32(11) || entity["pairz"] != int32(20) || entity["pairlead"] != int8(1) {
		t.Fatalf("pairing: %#v", entity)
	}
	if entity["CustomName"] != "x" || entity["LootTable"] != "loot_tables/chests/simple_dungeon.json" || entity["LootTableSeed"] != int64(3) {
		t.Fatalf("metadata: %#v", entity)
	}
	items, _ := entity["Items"].([]nbt.Compound)
	if len(items) != 1 || items[0]["Name"] != "minecraft:oak_log" || items[0]["Slot"] != int8(0) {
		t.Fatalf("items: %#v", entity["Items"])
	}
}

func TestConvertSingle(t *testing.T) {
	got, err := Convert(blockentities.Input{
		SourceState: `minecraft:trapped_chest ["facing"="north","type"="single","waterlogged"="false"]`,
		Base:        `minecraft:trapped_chest []`,
		WorldPos:    celler.BlockPos{},
		Source:      nbt.Compound{"id": "minecraft:trapped_chest"},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "Chest" {
		t.Fatalf("id: %v", entity["id"])
	}
	if _, exists := entity["pairx"]; exists {
		t.Fatalf("single chest paired: %#v", entity)
	}
}
