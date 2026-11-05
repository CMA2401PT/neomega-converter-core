package brewing_stand

import (
	"testing"

	"neomega-converter-core/be2je/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base: "minecraft:brewing_stand []",
		Source: nbt.Compound{
			"Items": []nbt.Compound{
				{"Name": "minecraft:oak_log", "Count": int8(1), "Slot": int8(0)},
				{"Name": "minecraft:stone", "Count": int8(2), "Slot": int8(1)},
				{"Name": "minecraft:dirt", "Count": int8(3), "Slot": int8(3)},
				{"Name": "minecraft:sand", "Count": int8(4), "Slot": int8(4)},
			},
			"FuelAmount": int8(12),
			"CookTime":   int32(300),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "minecraft:brewing_stand" || entity["Fuel"] != int8(12) || entity["BrewTime"] != int16(300) {
		t.Fatalf("fields: %#v", entity)
	}
	// Bedrock 0/1/3/4 映射到 Java 3/0/2/4。
	want := []int8{3, 0, 2, 4}
	list, _ := entity["Items"].([]nbt.Compound)
	if len(list) != len(want) {
		t.Fatalf("items: %#v", list)
	}
	for i, item := range list {
		if item["Slot"] != want[i] {
			t.Fatalf("slot %d = %v, want %d", i, item["Slot"], want[i])
		}
	}
}
