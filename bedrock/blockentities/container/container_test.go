package container

import (
	"reflect"
	"testing"

	"neomega-converter-core/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestCommon(t *testing.T) {
	input := blockentities.Input{
		Base: `minecraft:barrel ["facing"="up"]`,
		Source: nbt.Compound{
			"Items": []nbt.Compound{
				{"Name": "minecraft:oak_log", "Count": int8(4), "Slot": int8(2)},
				{"Name": "minecraft:diamond_sword", "Count": int8(1), "Slot": int8(3)},
			},
			"CustomName":    "§a箱子",
			"LootTable":     "loot_tables/chests/simple_dungeon.json",
			"LootTableSeed": int64(7),
		},
	}
	entity, err := Common(input, "minecraft:barrel")
	if err != nil {
		t.Fatal(err)
	}
	want := nbt.Compound{
		"id": "minecraft:barrel",
		"Items": []nbt.Compound{
			{"id": "minecraft:oak_log", "count": int32(4), "Slot": int8(2)},
			{"id": "minecraft:diamond_sword", "count": int32(1), "Slot": int8(3)},
		},
		"CustomName":    `{"text":"箱子","color":"green"}`,
		"LootTable":     "minecraft:chests/simple_dungeon",
		"LootTableSeed": int64(7),
	}
	if !reflect.DeepEqual(entity, want) {
		t.Fatalf("got %#v, want %#v", entity, want)
	}
}

func TestCommonMissingItems(t *testing.T) {
	entity, err := Common(blockentities.Input{Base: "minecraft:barrel []"}, "minecraft:barrel")
	if err != nil {
		t.Fatal(err)
	}
	if list, ok := entity["Items"].([]nbt.Compound); !ok || len(list) != 0 {
		t.Fatalf("Items: %#v", entity["Items"])
	}
}

func TestCommonRejectsBadItems(t *testing.T) {
	if _, err := Common(blockentities.Input{Source: nbt.Compound{"Items": "nope"}}, "minecraft:barrel"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLootPath(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"loot_tables/chests/simple_dungeon.json", "minecraft:chests/simple_dungeon"},
		{"chests/simple_dungeon.json", "minecraft:chests/simple_dungeon"},
		{"minecraft:chests/simple_dungeon", "minecraft:chests/simple_dungeon"},
		{"custom:path", "custom:path"},
	} {
		if got := LootPath(test.in); got != test.want {
			t.Fatalf("LootPath(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestConvertKeepsState(t *testing.T) {
	got, err := Convert("minecraft:hopper")(blockentities.Input{
		Base:   `minecraft:hopper ["enabled"="true","facing"="down"]`,
		Source: nbt.Compound{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != `minecraft:hopper ["enabled"="true","facing"="down"]` {
		t.Fatalf("state: %s", got.State)
	}
	if got.BlockEntity["id"] != "minecraft:hopper" {
		t.Fatalf("id: %v", got.BlockEntity["id"])
	}
}
