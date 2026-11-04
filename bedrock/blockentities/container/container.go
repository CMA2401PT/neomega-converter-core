// Package container 转换与容器方块实体共有的字段：物品、自定义名称、战利品表。
// 具体类型（箱子、熔炉、营火等）在自己的包或本包的规则里追加专属字段。
package container

import (
	"fmt"
	"strings"

	"neomega-converter-core/bedrock/blockentities"
	"neomega-converter-core/bedrock/internal/textcomponent"
	"neomega-converter-core/bedrock/items"
	nbt "neomega-nbt/presets/v1"
)

// Items 转换 Bedrock 的 Items 列表，保留原槽位并跳过转不出的条目。
func Items(source nbt.Compound) ([]nbt.Compound, error) {
	raw, exists := source["Items"]
	if !exists {
		return []nbt.Compound{}, nil
	}
	list, ok := nbt.Compounds(raw)
	if !ok {
		return nil, fmt.Errorf("container Items must be a compound list")
	}
	out := make([]nbt.Compound, 0, len(list))
	for _, item := range list {
		if converted := items.ConvertItem(item); converted != nil {
			out = append(out, converted)
		}
	}
	return out, nil
}

// Common 组装 Java 容器方块实体的公共字段。javaID 是目标 Java 方块实体 id。
func Common(input blockentities.Input, javaID string) (nbt.Compound, error) {
	list, err := Items(input.Source)
	if err != nil {
		return nil, err
	}
	entity := nbt.Compound{"id": javaID, "Items": list}
	if name, ok := input.Source["CustomName"].(string); ok && name != "" {
		entity["CustomName"] = textcomponent.Styled(name)
	}
	if loot, ok := input.Source["LootTable"].(string); ok && loot != "" {
		entity["LootTable"] = LootPath(loot)
	}
	if seed, ok := Number(input.Source["LootTableSeed"]); ok {
		entity["LootTableSeed"] = seed
	}
	return entity, nil
}

// LootPath 把 Bedrock 的战利品表路径换成 Java 的资源位置。
func LootPath(path string) string {
	path = strings.TrimPrefix(path, "loot_tables/")
	path = strings.TrimSuffix(path, ".json")
	if strings.ContainsRune(path, ':') {
		return path
	}
	return "minecraft:" + path
}

// Convert 生成“Java 方块实体 id 由调用方固定”的容器规则。
func Convert(javaID string) blockentities.ConvertFunc {
	return func(input blockentities.Input) (blockentities.Result, error) {
		entity, err := Common(input, javaID)
		if err != nil {
			return blockentities.Result{}, err
		}
		return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
	}
}

// Number 放宽 Bedrock 数值宽度，方块实体字段常在不同版本间改宽度。
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
