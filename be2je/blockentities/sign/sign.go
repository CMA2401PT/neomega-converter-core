package sign

import (
	"fmt"
	"strings"

	"neomega-converter-core/be2je/blockentities"
	"neomega-converter-core/be2je/internal/textcomponent"
	"neomega-converter-core/mapping"
	nbt "neomega-nbt/presets/v1"
)

const (
	ID        = "Sign"
	HangingID = "HangingSign"
)

func Convert(input blockentities.Input) (blockentities.Result, error) {
	front := input.Source
	if raw, exists := input.Source["FrontText"]; exists {
		var ok bool
		front, ok = raw.(nbt.Compound)
		if !ok {
			return blockentities.Result{}, fmt.Errorf("sign FrontText must be a compound")
		}
	}
	back, _ := input.Source["BackText"].(nbt.Compound)
	id := "minecraft:sign"
	if strings.HasSuffix(input.Base.NameWithNameSpace(), "_hanging_sign") {
		id = "minecraft:hanging_sign"
	}
	waxed, _ := input.Source["IsWaxed"].(int8)
	entity := nbt.Compound{
		"id":         id,
		"front_text": side(front),
		"back_text":  side(back),
		"is_waxed":   flag(waxed),
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}

func side(source nbt.Compound) nbt.Compound {
	text, _ := source["Text"].(string)
	lines := strings.Split(text, "\n")
	messages := make([]string, 4)
	for i := range messages {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		messages[i] = textcomponent.Styled(line)
	}
	color := "black"
	if argb, ok := source["SignTextColor"].(int32); ok {
		color = nearestDye(argb & 0xFFFFFF)
	}
	glow, _ := source["IgnoreLighting"].(int8)
	return nbt.Compound{
		"messages":         messages,
		"color":            color,
		"has_glowing_text": flag(glow),
	}
}

func nearestDye(rgb int32) string {
	name := "black"
	best := int32(3*255*255 + 1)
	for index, dye := range mapping.DyeRGB {
		r := (rgb>>16)&255 - (dye>>16)&255
		g := (rgb>>8)&255 - (dye>>8)&255
		b := rgb&255 - dye&255
		// 平方距离保留同样的大小关系，无需开方。
		distance := r*r + g*g + b*b
		if distance < best {
			best, name = distance, mapping.DyeNames[index]
		}
	}
	return name
}

func flag(value int8) int8 {
	if value != 0 {
		return 1
	}
	return 0
}
