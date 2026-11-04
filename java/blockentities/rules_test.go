package blockentities_test

import (
	"testing"

	"neomega-converter-core/java/blockentities"
	"neomega-converter-core/java/blockentities/beacon"
	"neomega-converter-core/java/blockentities/brewing_stand"
	"neomega-converter-core/java/blockentities/campfire"
	"neomega-converter-core/java/blockentities/command_block"
	"neomega-converter-core/java/blockentities/decorated_pot"
	"neomega-converter-core/java/blockentities/end_gateway"
	"neomega-converter-core/java/blockentities/jukebox"
	"neomega-converter-core/java/blockentities/lectern"
	"neomega-converter-core/java/blockentities/mob_spawner"
	"neomega-converter-core/java/blockentities/shulker_box"
	nbt "neomega-nbt/presets/v1"
)

func run(t *testing.T, fn blockentities.ConvertFunc, input blockentities.Input) nbt.Compound {
	t.Helper()
	got, err := fn(input)
	if err != nil {
		t.Fatal(err)
	}
	return got.BlockEntity
}

func TestShulkerBox(t *testing.T) {
	entity := run(t, shulker_box.Convert, blockentities.Input{
		SourceState: `minecraft:shulker_box ["facing"="east"]`,
		Base:        `minecraft:shulker_box []`,
		Source: nbt.Compound{"Items": []nbt.Compound{
			{"id": "minecraft:oak_log", "count": int32(1), "Slot": int8(0)},
		}},
	})
	if entity["id"] != "ShulkerBox" || entity["facing"] != int8(5) {
		t.Fatalf("%#v", entity)
	}
	if items, _ := entity["Items"].([]nbt.Compound); len(items) != 1 {
		t.Fatalf("items: %#v", entity["Items"])
	}
}

func TestJukebox(t *testing.T) {
	entity := run(t, jukebox.Convert, blockentities.Input{
		Source: nbt.Compound{"RecordItem": nbt.Compound{"id": "minecraft:diamond_sword", "count": int32(1)}},
	})
	if entity["id"] != "Jukebox" {
		t.Fatalf("%#v", entity)
	}
	record, _ := entity["RecordItem"].(nbt.Compound)
	if record["Name"] != "minecraft:diamond_sword" {
		t.Fatalf("record: %#v", record)
	}
}

func TestLectern(t *testing.T) {
	entity := run(t, lectern.Convert, blockentities.Input{
		Source: nbt.Compound{
			"Book": nbt.Compound{"id": "minecraft:written_book", "count": int32(1)},
			"Page": int32(3),
		},
	})
	if entity["id"] != "Lectern" || entity["hasBook"] != int8(1) || entity["page"] != int32(3) {
		t.Fatalf("%#v", entity)
	}
}

func TestBrewingStand(t *testing.T) {
	entity := run(t, brewing_stand.Convert, blockentities.Input{
		Source: nbt.Compound{
			"Items": []nbt.Compound{
				{"id": "minecraft:oak_log", "count": int32(1), "Slot": int8(0)},
				{"id": "minecraft:stone", "count": int32(1), "Slot": int8(3)},
			},
			"Fuel":     int8(7),
			"BrewTime": int16(200),
		},
	})
	if entity["id"] != "BrewingStand" || entity["FuelAmount"] != int8(7) || entity["CookTime"] != int32(200) {
		t.Fatalf("%#v", entity)
	}
	items, _ := entity["Items"].([]nbt.Compound)
	if len(items) != 2 || items[0]["Slot"] != int8(1) || items[1]["Slot"] != int8(0) {
		t.Fatalf("slots: %#v", items)
	}
}

func TestBeacon(t *testing.T) {
	entity := run(t, beacon.Convert, blockentities.Input{
		Source: nbt.Compound{"primary_effect": "minecraft:speed", "secondary_effect": "minecraft:regeneration"},
	})
	if entity["primary"] != int32(1) || entity["secondary"] != int32(10) {
		t.Fatalf("%#v", entity)
	}
}

func TestMobSpawner(t *testing.T) {
	entity := run(t, mob_spawner.Convert, blockentities.Input{
		Source: nbt.Compound{
			"Delay":     int16(20),
			"SpawnData": nbt.Compound{"entity": nbt.Compound{"id": "minecraft:zombified_piglin"}},
		},
	})
	if entity["Delay"] != int16(20) || entity["EntityIdentifier"] != "minecraft:zombie_pigman" {
		t.Fatalf("%#v", entity)
	}
}

func TestCampfire(t *testing.T) {
	entity := run(t, campfire.Convert, blockentities.Input{
		Source: nbt.Compound{
			"Items": []nbt.Compound{
				{"id": "minecraft:oak_log", "count": int32(1), "Slot": int8(0)},
				{"id": "minecraft:stone", "count": int32(1), "Slot": int8(2)},
			},
			"CookingTimes": nbt.Array([]int32{1, 2, 3, 4}),
		},
	})
	if entity["id"] != "Campfire" || entity["ItemTime2"] != int32(2) {
		t.Fatalf("%#v", entity)
	}
	if _, ok := entity["Item1"].(nbt.Compound); !ok {
		t.Fatalf("Item1: %#v", entity["Item1"])
	}
	if _, ok := entity["Item3"].(nbt.Compound); !ok {
		t.Fatalf("Item3: %#v", entity["Item3"])
	}
}

func TestCommandBlock(t *testing.T) {
	entity := run(t, command_block.Convert, blockentities.Input{
		Source: nbt.Compound{
			"Command": "say hi", "auto": int8(1), "SuccessCount": int32(3),
			"LastOutput": `{"text":"done","color":"green"}`,
		},
	})
	if entity["id"] != "CommandBlock" || entity["Command"] != "say hi" ||
		entity["SuccessCount"] != int32(3) || entity["LastOutput"] != "§adone" {
		t.Fatalf("%#v", entity)
	}
}

func TestEndGateway(t *testing.T) {
	entity := run(t, end_gateway.Convert, blockentities.Input{
		Source: nbt.Compound{"exit_portal": nbt.Array([]int32{1, 64, -2}), "Age": int32(40)},
	})
	if entity["id"] != "EndGateway" || entity["Age"] != int32(40) {
		t.Fatalf("%#v", entity)
	}
	portal, _ := entity["ExitPortal"].([]int32)
	if len(portal) != 3 || portal[1] != 64 {
		t.Fatalf("portal: %#v", entity["ExitPortal"])
	}
}

func TestDecoratedPot(t *testing.T) {
	entity := run(t, decorated_pot.Convert, blockentities.Input{
		Source: nbt.Compound{
			"sherds": []string{"minecraft:brick"},
			"item":   nbt.Compound{"id": "minecraft:oak_log", "count": int32(1)},
		},
	})
	if entity["id"] != "DecoratedPot" {
		t.Fatalf("%#v", entity)
	}
	if sherds, _ := entity["sherds"].([]string); len(sherds) != 1 {
		t.Fatalf("sherds: %#v", entity["sherds"])
	}
	if item, _ := entity["item"].(nbt.Compound); item["Name"] != "minecraft:oak_log" {
		t.Fatalf("item: %#v", entity["item"])
	}
}
