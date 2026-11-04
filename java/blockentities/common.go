package blockentities

import (
	"fmt"

	"neomega-converter-core/java/internal/textcomponent"
	"neomega-converter-core/java/items"
	"neomega-converter-core/mapping"
	nbt "neomega-nbt/presets/v1"
)

// Items 转换 Java 的 Items 列表，保留槽位并跳过转不出的条目。
func Items(input Input) ([]nbt.Compound, error) {
	raw, exists := input.Source["Items"]
	if !exists {
		return []nbt.Compound{}, nil
	}
	list, ok := nbt.Compounds(raw)
	if !ok {
		return nil, fmt.Errorf("Java Items must be a compound list")
	}
	out := make([]nbt.Compound, 0, len(list))
	for _, item := range list {
		if converted := items.ConvertItem(item, input.ReportItem); converted != nil {
			out = append(out, converted)
		}
	}
	return out, nil
}

// Common 组装 Java 容器方块实体共有的 Items、CustomName、LootTable 和 Seed。
func Common(input Input) (nbt.Compound, error) {
	list, err := Items(input)
	if err != nil {
		return nil, err
	}
	entity := nbt.Compound{"Items": list}
	if name, ok := input.Source["CustomName"].(string); ok && name != "" {
		entity["CustomName"] = textcomponent.ToSection(name)
	}
	if loot, ok := input.Source["LootTable"].(string); ok && loot != "" {
		entity["LootTable"] = mapping.LootTableToBedrock(loot)
	}
	if seed, ok := Number(input.Source["LootTableSeed"]); ok {
		entity["LootTableSeed"] = seed
	}
	return entity, nil
}

// Number 放宽 Java 数值宽度。
func Number(value any) (int64, bool) {
	switch value := value.(type) {
	case int8:
		return int64(value), true
	case int16:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	default:
		return 0, false
	}
}
