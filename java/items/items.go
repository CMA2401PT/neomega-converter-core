// Package items 把 Java 物品堆叠转成 Bedrock 物品 Compound。
// 它是 bedrock/items 的反方向：Java 1.21.4 的 components 要拆回 Bedrock 的
// Name/Damage/Block/tag 字段。
package items

import (
	"sort"
	"strings"

	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	javablocks "neomega-blocks/presets/java/v1"
	"neomega-converter-core/java/internal/textcomponent"
	"neomega-converter-core/mapping"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

// 与 java 包写出的目标方块状态版本一致。
const bedrockBlockVersion int32 = 1<<24 | 21<<16 | 50<<8

// ConvertItem 转换一个 Java 物品。无法转换时返回 nil，并把物品 id 交给 report。
// report 可以为 nil。
func ConvertItem(src nbt.Compound, report func(name string)) nbt.Compound {
	id, _ := src["id"].(string)
	count, _ := number(src["count"])
	if id == "" || count <= 0 {
		return nil
	}
	bedrock, data, ok := tables.BedrockItem(id)
	if !ok {
		notify(report, id)
		return nil
	}
	if count > 127 {
		count = 127
	}
	out := nbt.Compound{"Name": bedrock, "Count": int8(count)}
	if data != 0 {
		out["Damage"] = int16(data)
	}

	// 方块物品带 Block，才能保留 Java 里已经确定的变体。
	if _, isBlock := javablocks.BlockProperties(id); isBlock {
		if state, ok := converts.ConvertFromJava(ir.IRBlock(id + " []")); ok {
			out["Block"] = nbt.Compound{
				"name":    state.NameWithNameSpace(),
				"states":  state.States().GoMap(),
				"version": bedrockBlockVersion,
			}
		}
	}

	components, _ := src["components"].(nbt.Compound)
	if damage, ok := potionDamage(id, components); ok {
		out["Damage"] = damage
	}
	tag := convertComponents(id, components, report)
	if len(tag) > 0 {
		out["tag"] = tag
	}
	if slot, ok := src["Slot"].(int8); ok {
		out["Slot"] = slot
	}
	return out
}

func convertComponents(id string, components nbt.Compound, report func(string)) nbt.Compound {
	tag := nbt.Compound{}
	if components == nil {
		return tag
	}
	if value, ok := number(components["minecraft:damage"]); ok && value > 0 {
		tag["Damage"] = int32(value)
	}

	display := nbt.Compound{}
	if raw, exists := components["minecraft:custom_name"]; exists {
		if text := textcomponent.ToSection(raw); text != "" {
			display["Name"] = text
		}
	}
	if raw, exists := components["minecraft:lore"]; exists {
		lines := lore(raw)
		if len(lines) > 0 {
			display["Lore"] = lines
		}
	}
	if len(display) > 0 {
		tag["display"] = display
	}

	for _, key := range []string{"minecraft:enchantments", "minecraft:stored_enchantments"} {
		enchants := enchantments(components[key])
		if len(enchants) > 0 {
			tag["ench"] = enchants
		}
	}

	if raw, ok := components["minecraft:dyed_color"].(nbt.Compound); ok {
		if rgb, ok := number(raw["rgb"]); ok {
			tag["customColor"] = int32(rgb) | -16777216
		}
	}
	if _, ok := components["minecraft:unbreakable"]; ok {
		tag["Unbreakable"] = int8(1)
	}
	if value, ok := number(components["minecraft:repair_cost"]); ok && value > 0 {
		tag["RepairCost"] = int32(value)
	}

	if raw, exists := components["minecraft:container"]; exists {
		if items := container(raw, report); len(items) > 0 {
			tag["Items"] = items
		}
	}

	bookContent(id, components, tag)
	return tag
}

// potionDamage 把 Java 药水组件还原成 Bedrock 顶层的 Damage 编号。
func potionDamage(id string, components nbt.Compound) (int16, bool) {
	raw, exists := components["minecraft:potion_contents"]
	if !exists {
		return 0, false
	}
	compound, ok := raw.(nbt.Compound)
	if !ok {
		return 0, false
	}
	name, ok := compound["potion"].(string)
	if !ok {
		return 0, false
	}
	index, ok := mapping.PotionIndex(name)
	if !ok {
		return 0, false
	}
	if shortName(id) == "tipped_arrow" {
		index++
	}
	return int16(index), true
}

func enchantments(raw any) []nbt.Compound {
	compound, ok := raw.(nbt.Compound)
	if !ok {
		return nil
	}
	levels, ok := compound["levels"].(nbt.Compound)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(levels))
	for name := range levels {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]nbt.Compound, 0, len(names))
	for _, name := range names {
		id, ok := mapping.EnchantmentID(name)
		if !ok {
			continue
		}
		level, _ := number(levels[name])
		out = append(out, nbt.Compound{"id": id, "lvl": int16(level)})
	}
	return out
}

func lore(raw any) []string {
	var values []string
	switch raw := raw.(type) {
	case []string:
		values = raw
	case []any:
		for _, value := range raw {
			if text, ok := value.(string); ok {
				values = append(values, text)
			}
		}
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, textcomponent.ToSection(value))
	}
	return lines
}

func container(raw any, report func(string)) []nbt.Compound {
	list, ok := nbt.Compounds(raw)
	if !ok {
		return nil
	}
	out := make([]nbt.Compound, 0, len(list))
	for _, entry := range list {
		slot, _ := number(entry["slot"])
		item, ok := entry["item"].(nbt.Compound)
		if !ok {
			continue
		}
		converted := ConvertItem(item, report)
		if converted == nil {
			continue
		}
		converted["Slot"] = int8(slot)
		out = append(out, converted)
	}
	return out
}

func bookContent(id string, components nbt.Compound, tag nbt.Compound) {
	key := "minecraft:" + shortName(id) + "_content"
	content, ok := components[key].(nbt.Compound)
	if !ok {
		return
	}
	pages := []nbt.Compound{}
	if raw, exists := content["pages"]; exists {
		list, _ := nbt.Compounds(raw)
		for _, page := range list {
			text, _ := page["raw"].(string)
			pages = append(pages, nbt.Compound{"text": textcomponent.ToSection(text)})
		}
	}
	tag["pages"] = pages
	if title, ok := content["title"].(nbt.Compound); ok {
		tag["title"] = textcomponent.ToSection(title["raw"])
	}
	if author, ok := content["author"].(string); ok {
		tag["author"] = author
	}
	if generation, ok := number(content["generation"]); ok {
		tag["generation"] = int32(generation)
	}
}

func shortName(id string) string {
	return strings.TrimPrefix(id, "minecraft:")
}

func number(value any) (int32, bool) {
	switch value := value.(type) {
	case int8:
		return int32(value), true
	case int16:
		return int32(value), true
	case int32:
		return value, true
	default:
		return 0, false
	}
}

func notify(report func(string), name string) {
	if report != nil {
		report(name)
	}
}
