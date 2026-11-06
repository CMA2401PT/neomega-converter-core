package structure

import (
	"fmt"
	"maps"
	"neomega-blocks/ir"
	"slices"
	"strings"
)

// parseSchemPalette 解析 Sponge 状态文本到按下标排列的 Java 调色板。
func parseSchemPalette(raw map[string]any) ([]ir.IRBlock, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("schem: empty palette")
	}
	assigned := map[int32]ir.IRBlock{}
	maxIndex := int32(-1)
	for spec, value := range raw {
		index, ok := nbtInt32(value)
		if !ok || index < 0 {
			return nil, fmt.Errorf("schem: palette index for %q", spec)
		}
		block, err := parseState(spec)
		if err != nil {
			return nil, err
		}
		if _, exists := assigned[index]; exists {
			return nil, fmt.Errorf("schem: duplicate palette index %d", index)
		}
		assigned[index] = block
		if index > maxIndex {
			maxIndex = index
		}
	}
	palette := make([]ir.IRBlock, maxIndex+1)
	for index, block := range assigned {
		palette[index] = block
	}

	return palette, nil
}

// parseState 把 `name[key=value,key=value]` 收成 Java 状态。
// 原理图里的属性值都是文本，含水也是 "true" / "false"，不能按基岩的布尔来读。
func parseState(spec string) (ir.IRBlock, error) {
	spec = strings.TrimSpace(spec)
	name := spec
	props := map[string]string{}
	if start := strings.IndexByte(spec, '['); start >= 0 {
		if !strings.HasSuffix(spec, "]") {
			return "", fmt.Errorf("schem: block state %q", spec)
		}
		name = spec[:start]
		body := spec[start+1 : len(spec)-1]
		if body != "" {
			for part := range strings.SplitSeq(body, ",") {
				key, value, ok := strings.Cut(part, "=")
				if !ok || key == "" {
					return "", fmt.Errorf("schem: block state %q", spec)
				}
				props[key] = value
			}
		}
	}
	if name == "" {
		return "", fmt.Errorf("schem: block state %q", spec)
	}
	if !strings.Contains(name, ":") {
		name = "minecraft:" + name
	}
	if len(props) == 0 {
		block := ir.IRBlock(name + " []").Tidy()
		if !block.IsTidy() {
			return "", fmt.Errorf("schem: block state %q", spec)
		}
		return block, nil
	}
	keys := slices.Sorted(maps.Keys(props))
	var b strings.Builder
	b.WriteString(name)
	b.WriteString(" [")
	for i, key := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q=%q", key, props[key])
	}
	b.WriteByte(']')
	block := ir.IRBlock(b.String()).Tidy()
	if !block.IsTidy() {
		return "", fmt.Errorf("schem: block state %q", spec)
	}
	return block, nil
}
