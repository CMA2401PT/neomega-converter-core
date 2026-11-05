package beacon

import (
	"testing"

	"neomega-converter-core/be2je/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base:   "minecraft:beacon []",
		Source: nbt.Compound{"primary": int32(1), "secondary": int32(10)},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["primary_effect"] != "minecraft:speed" || entity["secondary_effect"] != "minecraft:regeneration" {
		t.Fatalf("effects: %#v", entity)
	}
}

func TestUnknownEffectIgnored(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base:   "minecraft:beacon []",
		Source: nbt.Compound{"primary": int32(2)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := got.BlockEntity["primary_effect"]; exists {
		t.Fatalf("unexpected effect: %#v", got.BlockEntity)
	}
}
