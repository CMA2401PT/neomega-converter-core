package banner

import (
	"testing"

	"neomega-converter-core/je2be/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		SourceState: `minecraft:red_wall_banner ["facing"="north"]`,
		Base:        `minecraft:red_wall_banner []`,
		Source: nbt.Compound{
			"id": "minecraft:banner",
			"patterns": []nbt.Compound{
				{"pattern": "minecraft:border", "color": "red"},
				{"pattern": "minecraft:globe", "color": "white"},
				{"pattern": "minecraft:unknown_pattern", "color": "black"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "Banner" || entity["Base"] != int32(1) {
		t.Fatalf("base: %#v", entity)
	}
	patterns, _ := entity["Patterns"].([]nbt.Compound)
	if len(patterns) != 2 || patterns[0]["Pattern"] != "bo" || patterns[0]["Color"] != int32(1) ||
		patterns[1]["Pattern"] != "glb" || patterns[1]["Color"] != int32(15) {
		t.Fatalf("patterns: %#v", entity["Patterns"])
	}
}
