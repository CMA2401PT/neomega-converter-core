// Package mapping 保存两个转换方向共用的固定对照表。
// 它不依赖 NBT 或存档类型，只放名称、编号和顺序。
package mapping

import "strings"

// Enchantments 是 Bedrock 附魔编号到 Java 名称的顺序表。
var Enchantments = []string{
	"protection", "fire_protection", "feather_falling", "blast_protection", "projectile_protection",
	"thorns", "respiration", "depth_strider", "aqua_affinity", "sharpness", "smite", "bane_of_arthropods",
	"knockback", "fire_aspect", "looting", "efficiency", "silk_touch", "unbreaking", "fortune", "power",
	"punch", "flame", "infinity", "luck_of_the_sea", "lure", "frost_walker", "mending", "binding_curse",
	"vanishing_curse", "impaling", "riptide", "loyalty", "channeling", "multishot", "piercing",
	"quick_charge", "soul_speed", "swift_sneak", "wind_burst", "density", "breach",
}

// Potions 是 Bedrock 药水编号到 Java 名称的顺序表。
var Potions = []string{
	"water", "mundane", "long_mundane", "thick", "awkward", "night_vision", "long_night_vision",
	"invisibility", "long_invisibility", "leaping", "long_leaping", "strong_leaping", "fire_resistance",
	"long_fire_resistance", "swiftness", "long_swiftness", "strong_swiftness", "slowness", "long_slowness",
	"water_breathing", "long_water_breathing", "healing", "strong_healing", "harming", "strong_harming",
	"poison", "long_poison", "strong_poison", "regeneration", "long_regeneration", "strong_regeneration",
	"strength", "long_strength", "strong_strength", "weakness", "long_weakness", "decay", "turtle_master",
	"long_turtle_master", "strong_turtle_master", "slow_falling", "long_slow_falling", "strong_slowness",
	"wind_charged", "weaving", "oozing", "infested",
}

// EnchantmentID 把 Java 附魔名转回 Bedrock 编号。
func EnchantmentID(name string) (int16, bool) {
	name = strings.TrimPrefix(name, "minecraft:")
	for index, enchantment := range Enchantments {
		if enchantment == name {
			return int16(index), true
		}
	}
	return 0, false
}

// PotionIndex 把 Java 药水名转回 Bedrock 的 Damage 编号。
func PotionIndex(name string) (int32, bool) {
	name = strings.TrimPrefix(name, "minecraft:")
	for index, potion := range Potions {
		if potion == name {
			return int32(index), true
		}
	}
	return 0, false
}
