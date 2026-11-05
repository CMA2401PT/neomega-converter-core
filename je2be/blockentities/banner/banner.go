// Package banner 把 Java 旗帜底色与图案转回 Bedrock。
package banner

import (
	"strings"

	"neomega-converter-core/je2be/blockentities"
	"neomega-converter-core/mapping"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

const ID = "minecraft:banner"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity := nbt.Compound{"id": "Banner", "Type": int32(0)}
	// Java 颜色在方块名里；Bedrock 的 Base 是 0 黑..15 白。
	color := strings.TrimSuffix(strings.TrimSuffix(input.SourceState.ShortName(), "_wall_banner"), "_banner")
	if index, ok := mapping.DyeIndex(color); ok {
		entity["Base"] = mapping.BedrockDye(index)
	}
	patterns := []nbt.Compound{}
	if raw, exists := input.Source["patterns"]; exists {
		list, _ := nbt.Compounds(raw)
		for _, layer := range list {
			name, _ := layer["pattern"].(string)
			code, ok := tables.BannerPatternCode(name)
			if !ok {
				code, ok = mapping.ExtraBannerPatterns[name]
			}
			if !ok {
				continue
			}
			index := 15
			if color, ok := layer["color"].(string); ok {
				if value, ok := mapping.DyeIndex(color); ok {
					index = value
				}
			}
			patterns = append(patterns, nbt.Compound{
				"Pattern": code,
				"Color":   mapping.BedrockDye(index),
			})
		}
	}
	entity["Patterns"] = patterns
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
