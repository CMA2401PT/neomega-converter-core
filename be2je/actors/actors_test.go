package actors

import (
	"reflect"
	"testing"

	nbt "neomega-nbt/presets/v1"
)

func TestConvertGeneric(t *testing.T) {
	source := nbt.Compound{
		"identifier": "minecraft:zombie",
		"Pos":        []float32{1, 64, -3},
		"Rotation":   []float32{90, 15},
		"Motion":     []float32{0, -0.5, 0},
		"UniqueID":   int64(42),
		"CustomName": "僵尸",
		"Tags":       []string{"tag"},
		"IsBaby":     int8(1),
		"Persistent": int8(1),
		"Fire":       int16(20),
		"Attributes": []nbt.Compound{{"Name": "minecraft:health", "Current": float32(12)}},
		"Armor": []nbt.Compound{
			{"Name": "minecraft:oak_log", "Count": int8(1)},
			{"Count": int8(0), "Name": ""},
			{"Count": int8(0), "Name": ""},
			{"Count": int8(0), "Name": ""},
		},
	}
	entity, known := Convert(source)
	if !known || entity == nil {
		t.Fatalf("known=%v entity=%v", known, entity)
	}
	if entity["id"] != "minecraft:zombie" {
		t.Fatalf("id: %v", entity["id"])
	}
	if !reflect.DeepEqual(entity["Pos"], []float64{1, 64, -3}) {
		t.Fatalf("Pos: %#v", entity["Pos"])
	}
	if !reflect.DeepEqual(entity["Rotation"], []float32{90, 15}) {
		t.Fatalf("Rotation: %#v", entity["Rotation"])
	}
	if !reflect.DeepEqual(entity["UUID"], nbt.Array([]int32{0, 0, 0, 42})) {
		t.Fatalf("UUID: %#v", entity["UUID"])
	}
	if entity["CustomName"] != `{"text":"僵尸"}` || entity["Age"] != int32(-24000) ||
		entity["PersistenceRequired"] != int8(1) || entity["Fire"] != int16(20) || entity["Health"] != float32(12) {
		t.Fatalf("fields: %#v", entity)
	}
	equipment, _ := entity["equipment"].(nbt.Compound)
	head, _ := equipment["head"].(nbt.Compound)
	if head["id"] != "minecraft:oak_log" || head["count"] != int32(1) {
		t.Fatalf("equipment: %#v", entity["equipment"])
	}
	if _, exists := head["Slot"]; exists {
		t.Fatalf("equipment kept Slot: %#v", head)
	}
}

func TestConvertItemEntity(t *testing.T) {
	entity, known := Convert(nbt.Compound{
		"identifier": "minecraft:item",
		"Pos":        []float32{0, 64, 0},
		"Item":       nbt.Compound{"Name": "minecraft:oak_log", "Count": int8(2), "Slot": int8(0)},
		"Age":        int16(5),
	})
	if !known || entity["id"] != "minecraft:item" {
		t.Fatalf("%v %v", known, entity)
	}
	item, _ := entity["Item"].(nbt.Compound)
	if item["id"] != "minecraft:oak_log" || item["count"] != int32(2) {
		t.Fatalf("Item: %#v", entity["Item"])
	}
	if _, exists := item["Slot"]; exists {
		t.Fatalf("item kept Slot: %#v", item)
	}
	if entity["Age"] != int16(5) || entity["PickupDelay"] != int16(0) {
		t.Fatalf("age: %#v", entity)
	}
}

func TestConvertBoats(t *testing.T) {
	for _, test := range []struct {
		kind    string
		variant int32
		want    string
	}{
		{"boat", 0, "minecraft:oak_boat"},
		{"boat", 7, "minecraft:bamboo_raft"},
		{"chest_boat", 1, "minecraft:spruce_chest_boat"},
		{"chest_boat", 7, "minecraft:bamboo_chest_raft"},
	} {
		entity, known := Convert(nbt.Compound{
			"identifier": "minecraft:" + test.kind,
			"Pos":        []float32{0, 64, 0},
			"Variant":    test.variant,
		})
		if !known || entity["id"] != test.want {
			t.Fatalf("%s/%d = %v, want %s", test.kind, test.variant, entity["id"], test.want)
		}
	}
}

func TestConvertClassifies(t *testing.T) {
	if entity, known := Convert(nbt.Compound{"identifier": "minecraft:arrow", "Pos": []float32{0, 0, 0}}); entity != nil || !known {
		t.Fatalf("skipped entity: %v %v", entity, known)
	}
	if entity, known := Convert(nbt.Compound{"identifier": "minecraft:zombie"}); entity != nil || !known {
		t.Fatalf("incomplete entity: %v %v", entity, known)
	}
	if entity, known := Convert(nbt.Compound{"identifier": "minecraft:not_a_real_entity", "Pos": []float32{0, 0, 0}}); entity != nil || known {
		t.Fatalf("unknown entity: %v %v", entity, known)
	}
}
