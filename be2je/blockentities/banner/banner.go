package banner

import (
	"fmt"
	"strings"

	"neomega-blocks/ir"
	"neomega-converter-core/be2je/blockentities"
	"neomega-converter-core/mapping"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

const ID string = "Banner"

// Bedrock 底色编号 0..15，0 为黑色、15 为白色。
var baseColors = [...]string{
	"black", "red", "green", "brown", "blue", "purple", "cyan", "light_gray",
	"gray", "pink", "lime", "yellow", "light_blue", "magenta", "orange", "white",
}

// Java 的染料顺序，0 为白色、15 为黑色；图案颜色按 15-Bedrock 编号取。
var dyes = [...]string{
	"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray",
	"light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black",
}

// extraPatternName 从共享表里反查 pmmp 缺失的图案代码。
func extraPatternName(code string) (string, bool) {
	for name, value := range mapping.ExtraBannerPatterns {
		if value == code {
			return name, true
		}
	}
	return "", false
}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// 旗帜底色使用 Bedrock 的染料编号，不能沿用基础映射的颜色。
	base, ok := input.Source["Base"].(int32)
	if !ok || base < 0 || int(base) >= len(baseColors) {
		return blockentities.Result{}, fmt.Errorf("banner Base must be an Int in 0..15")
	}
	suffix := "_banner"
	if strings.Contains(input.Base.NameWithNameSpace(), "wall") {
		suffix = "_wall_banner"
	}
	state := ir.IRBlock("minecraft:" + baseColors[base] + suffix + " " + string(input.Base.States())).Tidy()

	patterns, err := convertPatterns(input.Source)
	if err != nil {
		return blockentities.Result{}, err
	}
	entity := nbt.Compound{"id": "minecraft:banner", "patterns": patterns}
	return blockentities.Result{State: state, BlockEntity: entity}, nil
}

// convertPatterns 把 Bedrock 的 Pattern/Color 列表换成 Java 的图案名与染料名。
// 不认识的图案代码整条跳过，和 mcconv 一样保住旗帜本身。
func convertPatterns(source nbt.Compound) ([]nbt.Compound, error) {
	raw, exists := source["Patterns"]
	if !exists {
		return []nbt.Compound{}, nil
	}
	list, ok := nbt.Compounds(raw)
	if !ok {
		return nil, fmt.Errorf("banner Patterns must be a compound list")
	}
	patterns := make([]nbt.Compound, 0, len(list))
	for _, layer := range list {
		code, _ := layer["Pattern"].(string)
		if code == "" {
			continue
		}
		name, ok := tables.BannerPattern(code)
		if !ok {
			name, ok = extraPatternName(code)
		}
		if !ok {
			continue
		}
		color := int64(15)
		if value, ok := layer["Color"].(int32); ok && value >= 0 && value <= 15 {
			color = int64(value)
		}
		patterns = append(patterns, nbt.Compound{
			"pattern": name,
			"color":   dyes[15-color],
		})
	}
	return patterns, nil
}
