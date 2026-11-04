package furnace

import (
	"testing"

	"neomega-blocks/ir"
	"neomega-converter-core/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	for _, test := range []struct {
		base string
		id   string
	}{
		{"minecraft:furnace []", "minecraft:furnace"},
		{"minecraft:blast_furnace []", "minecraft:blast_furnace"},
		{"minecraft:smoker []", "minecraft:smoker"},
	} {
		got, err := Convert(blockentities.Input{
			Base: ir.IRBlock(test.base),
			Source: nbt.Compound{
				"BurnTime":     int16(120),
				"CookTime":     int32(40),
				"BurnDuration": int32(300),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		entity := got.BlockEntity
		if entity["id"] != test.id {
			t.Fatalf("id = %v, want %s", entity["id"], test.id)
		}
		if entity["lit_time_remaining"] != int16(120) || entity["cooking_time_spent"] != int16(40) ||
			entity["lit_total_time"] != int16(300) || entity["cooking_total_time"] != int16(200) {
			t.Fatalf("timers: %#v", entity)
		}
	}
}
