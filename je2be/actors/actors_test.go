package actors

import (
	"reflect"
	"testing"

	nbt "neomega-nbt/presets/v1"
)

func TestConvertZombie(t *testing.T) {
	source := nbt.Compound{
		"id":  "minecraft:zombie",
		"Pos": []float64{1, 64, -3}, "Rotation": []float32{90, 0}, "Motion": []float64{0, -0.5, 0},
		"UUID":                nbt.Array([]int32{0, 0, 7, 42}),
		"CustomName":          `{"text":"僵尸","color":"green"}`,
		"Tags":                []string{"tag"},
		"PersistenceRequired": int8(1),
		"Age":                 int32(-24000),
		"Health":              float32(12),
		"equipment": nbt.Compound{
			"head":     nbt.Compound{"id": "minecraft:oak_log", "count": int32(1)},
			"mainhand": nbt.Compound{"id": "minecraft:diamond_sword", "count": int32(1)},
		},
	}
	entity, id, known := Convert(source, nil)
	if !known || entity == nil {
		t.Fatalf("known=%v entity=%v", known, entity)
	}
	if entity["identifier"] != "minecraft:zombie" || entity["UniqueID"] != int64(uint64(7)<<32|42) {
		t.Fatalf("fields: %#v", entity)
	}
	if !reflect.DeepEqual(entity["Pos"], []float32{1, 64, -3}) {
		t.Fatalf("Pos: %#v", entity["Pos"])
	}
	if entity["CustomName"] != "§a僵尸" || entity["Persistent"] != int8(1) || entity["IsBaby"] != int8(1) {
		t.Fatalf("fields: %#v", entity)
	}
	definitions, _ := entity["definitions"].([]string)
	if len(definitions) != 1 || definitions[0] != "+minecraft:zombie" {
		t.Fatalf("definitions: %#v", entity["definitions"])
	}
	if entity["Persistent"] != int8(1) {
		t.Fatalf("persistent: %#v", entity["Persistent"])
	}
	if _, exists := entity["Attributes"]; exists {
		t.Fatalf("health attributes need an entity definition table: %#v", entity["Attributes"])
	}
	armor, _ := entity["Armor"].([]nbt.Compound)
	if len(armor) != 4 || armor[0]["Name"] != "minecraft:oak_log" || armor[1]["Name"] != "" {
		t.Fatalf("armor: %#v", entity["Armor"])
	}
	mainhand, _ := entity["Mainhand"].([]nbt.Compound)
	if len(mainhand) != 1 || mainhand[0]["Name"] != "minecraft:diamond_sword" {
		t.Fatalf("mainhand: %#v", entity["Mainhand"])
	}
	if id != [8]byte{7, 0, 0, 0, 42, 0, 0, 0} {
		t.Fatalf("actor id: %x", id)
	}
}

func TestConvertItemEntity(t *testing.T) {
	entity, _, known := Convert(nbt.Compound{
		"id":   "minecraft:item",
		"Pos":  []float64{0, 64, 0},
		"Item": nbt.Compound{"id": "minecraft:oak_log", "count": int32(2)},
		"Age":  int16(5),
	}, nil)
	if !known || entity["identifier"] != "minecraft:item" || entity["Age"] != int16(5) {
		t.Fatalf("%v %#v", known, entity)
	}
	item, _ := entity["Item"].(nbt.Compound)
	if item["Name"] != "minecraft:oak_log" || item["Count"] != int8(2) {
		t.Fatalf("Item: %#v", entity["Item"])
	}
}

func TestConvertBoats(t *testing.T) {
	for _, test := range []struct {
		java string
		kind string
		want int32
	}{
		{"minecraft:oak_boat", "minecraft:boat", 0},
		{"minecraft:bamboo_raft", "minecraft:boat", 7},
		{"minecraft:spruce_chest_boat", "minecraft:chest_boat", 1},
		{"minecraft:bamboo_chest_raft", "minecraft:chest_boat", 7},
	} {
		entity, _, _ := Convert(nbt.Compound{"id": test.java, "Pos": []float64{0, 64, 0}}, nil)
		if entity["identifier"] != test.kind || entity["Variant"] != test.want {
			t.Fatalf("%s: %#v", test.java, entity)
		}
	}
}

func TestConvertClassifies(t *testing.T) {
	if entity, _, known := Convert(nbt.Compound{"id": "minecraft:arrow", "Pos": []float64{0, 0, 0}}, nil); entity != nil || !known {
		t.Fatal("arrow not skipped")
	}
	if entity, _, known := Convert(nbt.Compound{"id": "minecraft:not_a_real_entity", "Pos": []float64{0, 0, 0}}, nil); entity != nil || known {
		t.Fatal("unknown entity accepted")
	}
}

func TestConvertPassengers(t *testing.T) {
	actors, known := ConvertAll(nbt.Compound{
		"id": "minecraft:chicken", "Pos": []float64{0, 64, 0},
		"Passengers": []nbt.Compound{{"id": "minecraft:zombie", "Pos": []float64{0, 64.5, 0}}},
	}, nil)
	if !known || len(actors) != 2 {
		t.Fatalf("actors: %d, known=%v", len(actors), known)
	}
	if actors[0].NBT["identifier"] != "minecraft:chicken" || actors[1].NBT["identifier"] != "minecraft:zombie" {
		t.Fatalf("actors: %#v", actors)
	}
}

func TestConvertSheepAndHorse(t *testing.T) {
	sheep, _, _ := Convert(nbt.Compound{
		"id": "minecraft:sheep", "Pos": []float64{0, 64, 0},
		"Color": int32(14), "Sheared": int8(1),
	}, nil)
	if sheep["Color"] != int8(14) || sheep["Sheared"] != int8(1) {
		t.Fatalf("sheep: %#v", sheep)
	}
	horse, _, _ := Convert(nbt.Compound{
		"id": "minecraft:horse", "Pos": []float64{0, 64, 0}, "Variant": int32(0x0102),
	}, nil)
	if horse["Variant"] != int32(2) || horse["MarkVariant"] != int32(1) {
		t.Fatalf("horse: %#v", horse)
	}
	definitions, _ := horse["definitions"].([]string)
	if len(definitions) != 3 || definitions[1] != "+minecraft:base_chestnut" || definitions[2] != "+minecraft:markings_white_details" {
		t.Fatalf("definitions: %#v", definitions)
	}
}

func TestConvertPainting(t *testing.T) {
	painting, _, _ := Convert(nbt.Compound{
		"id": "minecraft:painting", "Pos": []float64{0, 64, 0},
		"variant": "minecraft:donkey_kong", "facing": int32(2),
	}, nil)
	if painting["Motif"] != "DonkeyKong" || painting["Direction"] != int8(2) {
		t.Fatalf("painting: %#v", painting)
	}
	if blockPos, _ := painting["block_pos"].([]int32); len(blockPos) != 3 {
		t.Fatalf("block_pos: %#v", painting["block_pos"])
	}
}

func TestConvertChestMinecart(t *testing.T) {
	entity, _, _ := Convert(nbt.Compound{
		"id": "minecraft:chest_minecart", "Pos": []float64{0, 64, 0},
		"Items":         []nbt.Compound{{"id": "minecraft:oak_log", "count": int32(2), "Slot": int8(0)}},
		"LootTable":     "minecraft:chests/abandoned_mineshaft",
		"LootTableSeed": int64(3),
	}, nil)
	if entity["LootTable"] != "loot_tables/chests/abandoned_mineshaft.json" {
		t.Fatalf("loot: %#v", entity["LootTable"])
	}
	items, _ := entity["ChestItems"].([]nbt.Compound)
	if len(items) != 1 || items[0]["Name"] != "minecraft:oak_log" {
		t.Fatalf("items: %#v", entity["ChestItems"])
	}
}
