package furnace

import (
	"testing"

	"neomega-converter-core/java/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		SourceState: `minecraft:blast_furnace ["facing"="north","lit"="false"]`,
		Base:        `minecraft:blast_furnace []`,
		Source: nbt.Compound{
			"id":                 "minecraft:blast_furnace",
			"lit_time_remaining": int16(120),
			"cooking_time_spent": int16(40),
			"lit_total_time":     int16(300),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "BlastFurnace" || entity["BurnTime"] != int16(120) ||
		entity["CookTime"] != int16(40) || entity["BurnDuration"] != int16(300) {
		t.Fatalf("fields: %#v", entity)
	}
}
