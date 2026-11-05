package campfire

import (
	"testing"

	"neomega-converter-core/be2je/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base: "minecraft:soul_campfire []",
		Source: nbt.Compound{
			"Item1":     nbt.Compound{"Name": "minecraft:oak_log", "Count": int8(1)},
			"Item3":     nbt.Compound{"Name": "minecraft:stone", "Count": int8(2)},
			"ItemTime1": int32(100),
			"ItemTime3": int32(300),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "minecraft:soul_campfire" {
		t.Fatalf("id: %v", entity["id"])
	}
	list, _ := entity["Items"].([]nbt.Compound)
	if len(list) != 2 || list[0]["Slot"] != int8(0) || list[1]["Slot"] != int8(2) {
		t.Fatalf("items: %#v", list)
	}
	times := entity["CookingTimes"]
	if times != nbt.Array([]int32{100, 0, 300, 0}) {
		t.Fatalf("times: %#v", times)
	}
	totals := entity["CookingTotalTimes"]
	if totals != nbt.Array([]int32{600, 600, 600, 600}) {
		t.Fatalf("totals: %#v", totals)
	}
}
