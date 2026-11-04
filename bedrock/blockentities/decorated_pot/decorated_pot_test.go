package decorated_pot

import (
	"testing"

	"neomega-converter-core/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base: "minecraft:decorated_pot []",
		Source: nbt.Compound{
			"sherds": []string{"minecraft:angler_pottery_sherd", "minecraft:brick"},
			"item":   nbt.Compound{"Name": "minecraft:oak_log", "Count": int8(1), "Slot": int8(0)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "minecraft:decorated_pot" {
		t.Fatalf("id: %v", entity["id"])
	}
	want := []string{"minecraft:angler_pottery_sherd", "minecraft:brick"}
	if got := entity["sherds"]; len(got.([]string)) != len(want) {
		t.Fatalf("sherds: %#v", got)
	}
	item, _ := entity["item"].(nbt.Compound)
	if item["id"] != "minecraft:oak_log" || item["count"] != int32(1) {
		t.Fatalf("item: %#v", entity["item"])
	}
	if _, exists := item["Slot"]; exists {
		t.Fatalf("item kept Slot: %#v", item)
	}
}
