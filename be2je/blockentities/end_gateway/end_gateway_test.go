package end_gateway

import (
	"testing"

	"neomega-converter-core/be2je/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base:   "minecraft:end_gateway []",
		Source: nbt.Compound{"ExitPortal": []int32{1, 64, -2}, "Age": int32(40)},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "minecraft:end_gateway" {
		t.Fatalf("id: %v", entity["id"])
	}
	if entity["exit_portal"] != nbt.Array([]int32{1, 64, -2}) {
		t.Fatalf("portal: %#v", entity["exit_portal"])
	}
	if entity["Age"] != int64(40) {
		t.Fatalf("age: %#v", entity["Age"])
	}
}

func TestOddPortalIgnored(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base:   "minecraft:end_gateway []",
		Source: nbt.Compound{"ExitPortal": []int32{1, 64}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := got.BlockEntity["exit_portal"]; exists {
		t.Fatalf("unexpected portal: %#v", got.BlockEntity)
	}
}
