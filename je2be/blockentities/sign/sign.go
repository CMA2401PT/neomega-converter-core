// Package sign 把 Java 告示牌的双面文字转回 Bedrock。
package sign

import (
	"strings"

	"neomega-converter-core/je2be/blockentities"
	"neomega-converter-core/je2be/internal/textcomponent"
	"neomega-converter-core/mapping"
	nbt "neomega-nbt/presets/v1"
)

const (
	ID        = "minecraft:sign"
	HangingID = "minecraft:hanging_sign"
)

func Convert(input blockentities.Input) (blockentities.Result, error) {
	id := "Sign"
	if input.SourceState.ShortName() == "hanging_sign" {
		id = "HangingSign"
	}
	entity := nbt.Compound{
		"id":        id,
		"FrontText": side(input.Source["front_text"]),
		"BackText":  side(input.Source["back_text"]),
		"IsWaxed":   int8(0),
	}
	if waxed, ok := input.Source["is_waxed"].(int8); ok {
		entity["IsWaxed"] = waxed
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}

// side 把 Java 的 messages/color/has_glowing_text 合成 Bedrock 的一面文字。
func side(raw any) nbt.Compound {
	text := ""
	color := "black"
	glow := int8(0)
	if compound, ok := raw.(nbt.Compound); ok {
		if messages, ok := compound["messages"].([]string); ok {
			parts := make([]string, 0, len(messages))
			for _, message := range messages {
				parts = append(parts, textcomponent.ToSection(message))
			}
			text = strings.TrimRight(strings.Join(parts, "\n"), "\n")
		}
		if value, ok := compound["color"].(string); ok && value != "" {
			color = value
		}
		if value, ok := compound["has_glowing_text"].(int8); ok {
			glow = value
		}
	}
	argb := int32(-16777216)
	if rgb, ok := mapping.DyeRGBValue(color); ok {
		argb = rgb | -16777216
	}
	return nbt.Compound{
		"Text": text, "SignTextColor": argb, "IgnoreLighting": glow,
		"HideGlowOutline": int8(0), "PersistFormatting": int8(1), "TextOwner": "",
	}
}
