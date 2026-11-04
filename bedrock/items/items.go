package items

import (
	"strings"

	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter-core/bedrock/internal/blockstate"
	"neomega-converter-core/bedrock/internal/textcomponent"
	tables "neomega-converter-core/third_party"
	nbt "neomega-nbt/presets/v1"
)

var enchantmentIDs = []string{
	"protection", "fire_protection", "feather_falling", "blast_protection", "projectile_protection",
	"thorns", "respiration", "depth_strider", "aqua_affinity", "sharpness", "smite", "bane_of_arthropods",
	"knockback", "fire_aspect", "looting", "efficiency", "silk_touch", "unbreaking", "fortune", "power",
	"punch", "flame", "infinity", "luck_of_the_sea", "lure", "frost_walker", "mending", "binding_curse",
	"vanishing_curse", "impaling", "riptide", "loyalty", "channeling", "multishot", "piercing",
	"quick_charge", "soul_speed", "swift_sneak", "wind_burst", "density", "breach",
}

var potionIDs = []string{
	"water", "mundane", "long_mundane", "thick", "awkward", "night_vision", "long_night_vision",
	"invisibility", "long_invisibility", "leaping", "long_leaping", "strong_leaping", "fire_resistance",
	"long_fire_resistance", "swiftness", "long_swiftness", "strong_swiftness", "slowness", "long_slowness",
	"water_breathing", "long_water_breathing", "healing", "strong_healing", "harming", "strong_harming",
	"poison", "long_poison", "strong_poison", "regeneration", "long_regeneration", "strong_regeneration",
	"strength", "long_strength", "strong_strength", "weakness", "long_weakness", "decay", "turtle_master",
	"long_turtle_master", "strong_turtle_master", "slow_falling", "long_slow_falling", "strong_slowness",
	"wind_charged", "weaving", "oozing", "infested",
}

// Bedrock 在更新中改过少数物品名，旧存档仍可能带旧名。
// 只列出改名后 Java 侧没有同名的条目；其余名字交给生成表处理。
var bedrockRenames = map[string]string{
	"minecraft:scute": "minecraft:turtle_scute",
}

// ConvertItem 把一个 Bedrock 物品 compound 转成 Java 物品 compound。
// 无法转换时返回 nil，调用方跳过整个物品。
func ConvertItem(src nbt.Compound) nbt.Compound {
	name, _ := src["Name"].(string)
	count, _ := src["Count"].(int8)
	if name == "" || name == "minecraft:air" || count <= 0 {
		return nil
	}
	tag, _ := src["tag"].(nbt.Compound)
	if tag == nil {
		tag = nbt.Compound{}
	}
	aux, _ := number(src["Damage"])
	javaID, blockItem := resolveID(src, name, aux)
	if javaID == "" {
		return nil
	}
	comps, ok := components(tag, javaID, aux, blockItem)
	if !ok {
		return nil
	}
	out := nbt.Compound{"id": javaID, "count": int32(count)}
	if len(comps) > 0 {
		out["components"] = comps
	}
	if slot, ok := src["Slot"].(int8); ok {
		out["Slot"] = slot
	}
	return out
}

// resolveID 先用方块表解释块物品，再处理表里的物品名。
// 块物品优先走 neomega-blocks，避免同一种方块出现两套命名。
func resolveID(src nbt.Compound, name string, aux int32) (javaID string, blockItem bool) {
	if block, ok := itemBlock(src, name, aux); ok {
		return block.NameWithNameSpace(), true
	}
	// 药水箭在 Bedrock 里仍是普通箭加 Damage，生成表只保存了普通箭。
	if shortName(name) == "arrow" && aux > 0 {
		return "minecraft:tipped_arrow", false
	}
	if renamed, ok := bedrockRenames[name]; ok {
		name = renamed
	}
	if java, ok := tables.Item(name, aux); ok {
		return java, false
	}
	// 唱片在旧 Bedrock 存档里叫 record_*，Java 侧是 music_disc_*。
	if record, ok := strings.CutPrefix(name, "minecraft:record_"); ok {
		if java, ok := tables.Item("minecraft:music_disc_"+record, aux); ok {
			return java, false
		}
	}
	// 表里没有的名字仍按已知类型兜底，换表前后行为保持稳定。
	switch shortName(name) {
	case "potion", "splash_potion", "lingering_potion", "written_book", "writable_book":
		return "minecraft:" + shortName(name), false
	}
	return "", false
}

// components 组装 Java 1.21.4 的物品组件。第二个返回值为 false 表示数据非法，
// 调用方应丢弃整个物品，避免留下不完整的堆叠。
func components(tag nbt.Compound, javaID string, aux int32, blockItem bool) (nbt.Compound, bool) {
	comps := nbt.Compound{}
	switch shortName(javaID) {
	case "potion", "splash_potion", "lingering_potion", "tipped_arrow":
		index := aux
		if shortName(javaID) == "tipped_arrow" {
			index--
		}
		if index < 0 || int(index) >= len(potionIDs) || potionIDs[index] == "decay" {
			return nil, false
		}
		comps["minecraft:potion_contents"] = nbt.Compound{"potion": "minecraft:" + potionIDs[index]}
	}

	if raw, exists := tag["ench"]; exists {
		enchants, ok := nbt.Compounds(raw)
		if !ok {
			return nil, false
		}
		levels := nbt.Compound{}
		for _, enchant := range enchants {
			id, idOK := enchant["id"].(int16)
			level, levelOK := enchant["lvl"].(int16)
			if !idOK || !levelOK || id < 0 || int(id) >= len(enchantmentIDs) {
				return nil, false
			}
			levels["minecraft:"+enchantmentIDs[id]] = int32(level)
		}
		if len(levels) > 0 {
			// 附魔书的附魔存放在 stored_enchantments，普通物品用 enchantments。
			key := "minecraft:enchantments"
			if shortName(javaID) == "enchanted_book" {
				key = "minecraft:stored_enchantments"
			}
			comps[key] = nbt.Compound{"levels": levels}
		}
	}

	// 耐久只对可损耗的非方块物品有意义；块物品的 Damage 是变体编号或旧字段。
	if !blockItem && damageable(javaID) {
		if damage, ok := number(tag["Damage"]); ok && damage > 0 {
			comps["minecraft:damage"] = damage
		}
	}
	if display, ok := tag["display"].(nbt.Compound); ok {
		if name, ok := display["Name"].(string); ok && name != "" {
			comps["minecraft:custom_name"] = textcomponent.Styled(name)
		}
		if lore, ok := display["Lore"].([]string); ok && len(lore) > 0 {
			lines := make([]string, 0, len(lore))
			for _, line := range lore {
				lines = append(lines, textcomponent.Styled(line))
			}
			comps["minecraft:lore"] = lines
		}
	}
	if color, ok := number(tag["customColor"]); ok {
		comps["minecraft:dyed_color"] = nbt.Compound{"rgb": color & 0xFFFFFF}
	}
	if value, ok := tag["Unbreakable"].(int8); ok && value != 0 {
		comps["minecraft:unbreakable"] = nbt.Compound{}
	}
	if cost, ok := number(tag["RepairCost"]); ok && cost > 0 {
		comps["minecraft:repair_cost"] = cost
	}

	if shortName(javaID) == "written_book" || shortName(javaID) == "writable_book" {
		content := bookContent(tag, shortName(javaID) == "written_book")
		if content == nil {
			return nil, false
		}
		comps["minecraft:"+shortName(javaID)+"_content"] = content
	}

	// 嵌套容器递归转换；转不出的条目跳过，但容器本身保留。
	if raw, exists := tag["Items"]; exists {
		nested, ok := nbt.Compounds(raw)
		if !ok {
			return nil, false
		}
		container := make([]nbt.Compound, 0, len(nested))
		for _, item := range nested {
			converted := ConvertItem(item)
			if converted == nil {
				continue
			}
			slot, _ := item["Slot"].(int8)
			delete(converted, "Slot")
			container = append(container, nbt.Compound{"slot": int32(slot), "item": converted})
		}
		if len(container) > 0 {
			comps["minecraft:container"] = container
		}
	}
	return comps, true
}

func itemBlock(src nbt.Compound, name string, aux int32) (ir.IRBlock, bool) {
	// Block 保存了明确的变体；缺少它时才解释 aux，最后用名称补齐默认状态。
	block, _ := src["Block"].(nbt.Compound)
	blockName, nameOK := block["name"].(string)
	states, statesOK := block["states"].(nbt.Compound)
	var source ir.IRBlock
	var ok bool
	if nameOK && statesOK {
		var err error
		source, err = ir.NewIRBlockFromGo(blockName, states)
		ok = err == nil
	} else {
		source, ok = converts.BedrockDataValueToBlock(name, uint32(aux))
	}
	if ok {
		if converted, ok := blockstate.ToJava(source); ok {
			return converted, true
		}
	}
	return blockstate.ToJava(ir.IRBlock(name + " []"))
}

func bookContent(tag nbt.Compound, written bool) nbt.Compound {
	pages := []nbt.Compound{}
	if raw, exists := tag["pages"]; exists {
		source, ok := nbt.Compounds(raw)
		if !ok {
			return nil
		}
		for _, page := range source {
			text, ok := page["text"].(string)
			if !ok {
				return nil
			}
			if written {
				text = textcomponent.Styled(text)
			}
			pages = append(pages, nbt.Compound{"raw": text})
		}
	}
	content := nbt.Compound{"pages": pages}
	if written {
		title, _ := tag["title"].(string)
		author, _ := tag["author"].(string)
		generation, _ := number(tag["generation"])
		content["title"] = nbt.Compound{"raw": title}
		content["author"] = author
		content["generation"] = generation
		content["resolved"] = int8(1)
	}
	return content
}

// damageable 排除药水、书和箭这类没有耐久的物品，它们的 tag.Damage 另有含义。
func damageable(javaID string) bool {
	switch shortName(javaID) {
	case "potion", "splash_potion", "lingering_potion", "tipped_arrow", "arrow",
		"written_book", "writable_book", "enchanted_book", "knowledge_book":
		return false
	}
	return true
}

func shortName(name string) string {
	return strings.TrimPrefix(name, "minecraft:")
}

func number(value any) (int32, bool) {
	// Bedrock 的辅助值和书本代数使用不同整数宽度。
	switch value := value.(type) {
	case int16:
		return int32(value), true
	case int32:
		return value, true
	default:
		return 0, false
	}
}
