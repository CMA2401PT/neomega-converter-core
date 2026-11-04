package mapping

import "strings"

// entityJavaIDs 把 Bedrock 实体名映射到 Java 名；未列出的实体两版同名。
var entityJavaIDs = map[string]string{
	"zombie_pigman":       "zombified_piglin",
	"evocation_illager":   "evoker",
	"villager_v2":         "villager",
	"zombie_villager_v2":  "zombie_villager",
	"tropicalfish":        "tropical_fish",
	"ender_crystal":       "end_crystal",
	"xp_orb":              "experience_orb",
	"fireworks_rocket":    "firework_rocket",
	"eye_of_ender_signal": "eye_of_ender",
	"thrown_trident":      "trident",
	"xp_bottle":           "experience_bottle",
}

var bedrockEntityIDs = func() map[string]string {
	out := make(map[string]string, len(entityJavaIDs))
	for bedrock, java := range entityJavaIDs {
		out[java] = bedrock
	}
	return out
}()

// JavaEntityID 返回 Bedrock 实体名对应的 Java 名。
func JavaEntityID(bedrock string) (string, bool) {
	bedrock = strings.TrimPrefix(bedrock, "minecraft:")
	java, ok := entityJavaIDs[bedrock]
	return java, ok
}

// BedrockEntityID 返回 Java 实体名对应的 Bedrock 名；没有特例时同名。
func BedrockEntityID(java string) string {
	java = strings.TrimPrefix(java, "minecraft:")
	if bedrock, ok := bedrockEntityIDs[java]; ok {
		return bedrock
	}
	return java
}

// skipEntities 是不转换的瞬时实体，键不带命名空间。
var skipEntities = map[string]bool{
	"arrow": true, "spectral_arrow": true, "trident": true, "snowball": true, "egg": true,
	"ender_pearl": true, "potion": true, "splash_potion": true, "lingering_potion": true,
	"fireball": true, "small_fireball": true, "dragon_fireball": true, "wither_skull": true,
	"shulker_bullet": true, "llama_spit": true, "fishing_bobber": true, "firework_rocket": true,
	"eye_of_ender": true, "experience_bottle": true, "area_effect_cloud": true, "lightning_bolt": true,
	"marker": true, "block_display": true, "item_display": true, "text_display": true, "interaction": true,
	"wind_charge": true, "breeze_wind_charge": true, "evoker_fangs": true, "player": true,
	"fishing_hook": true, "xp_bottle": true, "thrown_trident": true, "fireworks_rocket": true,
	"eye_of_ender_signal": true, "wither_skull_dangerous": true, "tripod_camera": true, "agent": true, "npc": true,
}

// SkipEntity 报告一个实体名（两版之一）是否属于瞬时实体。
func SkipEntity(name string) bool {
	return skipEntities[strings.TrimPrefix(name, "minecraft:")]
}

// BoatWoods 是 Bedrock Variant 编号对应的木头；下标的 `_raft` 由 bamboo 决定。
var BoatWoods = []string{"oak", "spruce", "birch", "jungle", "acacia", "dark_oak", "mangrove", "bamboo", "cherry", "pale_oak"}
