package sign

import (
	"testing"

	"neomega-converter-core/je2be/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	got, err := Convert(blockentities.Input{
		SourceState: `minecraft:oak_sign ["rotation"="0","waterlogged"="false"]`,
		Base:        `minecraft:oak_sign []`,
		Source: nbt.Compound{
			"id": "minecraft:sign",
			"front_text": nbt.Compound{
				"messages":         []string{`{"text":"首行","color":"green"}`, `{"text":""}`, `{"text":""}`, `{"text":""}`},
				"color":            "red",
				"has_glowing_text": int8(1),
			},
			"is_waxed": int8(1),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := got.BlockEntity
	if entity["id"] != "Sign" || entity["IsWaxed"] != int8(1) {
		t.Fatalf("fields: %#v", entity)
	}
	front, _ := entity["FrontText"].(nbt.Compound)
	if front["Text"] != "§a首行" || front["IgnoreLighting"] != int8(1) {
		t.Fatalf("front: %#v", front)
	}
	// 红色染料是 0xB02E26，带不透明 alpha。
	if front["SignTextColor"] != int32(0xB02E26)|-16777216 {
		t.Fatalf("colour: %#v", front["SignTextColor"])
	}
}
