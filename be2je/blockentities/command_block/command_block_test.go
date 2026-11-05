package command_block

import (
	"testing"

	"neomega-converter-core/be2je/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base: "minecraft:command_block []",
		Source: nbt.Compound{
			"Command": "say hi", "auto": int8(1), "powered": int8(0),
			"TrackOutput": int8(1), "SuccessCount": int32(3),
			"conditionMet": int8(1), "LastOutput": "done",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "minecraft:command_block" || entity["Command"] != "say hi" ||
		entity["auto"] != int8(1) || entity["SuccessCount"] != int32(3) || entity["LastOutput"] != `{"text":"done"}` {
		t.Fatalf("fields: %#v", entity)
	}
}
