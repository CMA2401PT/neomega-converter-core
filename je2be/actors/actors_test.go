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
			"head": nbt.Compound{"id": "minecraft:oak_log", "count": int32(1)},
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
	attributes, _ := entity["Attributes"].([]nbt.Compound)
	if len(attributes) != 1 || attributes[0]["Name"] != "minecraft:health" || attributes[0]["Current"] != float32(12) {
		t.Fatalf("attributes: %#v", entity["Attributes"])
	}
	equipment, _ := entity["equipment"].(nbt.Compound)
	armor, _ := equipment["Armor"].(nbt.Compound)
	if head, _ := armor["Head"].(nbt.Compound); head["Name"] != "minecraft:oak_log" {
		t.Fatalf("equipment: %#v", equipment)
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
