package mapping

import "strings"

// DyeNames 是 Java 的染料顺序：0 白色，15 黑色。
var DyeNames = []string{
	"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray",
	"light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black",
}

// DyeRGB 与 DyeNames 同序，值为不含 alpha 的 RGB。
var DyeRGB = []int32{
	0xF9FFFE, 0xF9801D, 0xC74EBD, 0x3AB3DA, 0xFED83D, 0x80C71F, 0xF38BAA, 0x474F52,
	0x9D9D97, 0x169C9C, 0x8932B8, 0x3C44AA, 0x835432, 0x5E7C16, 0xB02E26, 0x1D1D21,
}

// DyeIndex 返回 Java 染料名在 DyeNames 中的下标。
func DyeIndex(name string) (int, bool) {
	name = strings.TrimPrefix(name, "minecraft:")
	for index, dye := range DyeNames {
		if dye == name {
			return index, true
		}
	}
	return 0, false
}

// DyeRGBValue 返回染料名对应的 RGB。
func DyeRGBValue(name string) (int32, bool) {
	index, ok := DyeIndex(name)
	if !ok {
		return 0, false
	}
	return DyeRGB[index], true
}

// BedrockDye 把 Java 染料下标转成 Bedrock 的编号：0 黑色，15 白色。
func BedrockDye(javaIndex int) int32 { return int32(15 - javaIndex) }

// JavaDye 把 Bedrock 编号（0 黑，15 白）转成 Java 下标。
func JavaDye(bedrockIndex int32) int { return 15 - int(bedrockIndex) }

// ExtraBannerPatterns 是 pmmp 表里缺失的图案：Java 名 → Bedrock 代码。
var ExtraBannerPatterns = map[string]string{
	"minecraft:globe":  "glb",
	"minecraft:piglin": "pig",
	"minecraft:flow":   "flw",
	"minecraft:guster": "gus",
}

// LootTableToBedrock 把 Java 资源位置换回 Bedrock 的 JSON 路径。
func LootTableToBedrock(path string) string {
	path = strings.TrimPrefix(path, "minecraft:")
	return "loot_tables/" + path + ".json"
}

// LootTableToJava 把 Bedrock 的 JSON 路径换成 Java 资源位置。
func LootTableToJava(path string) string {
	path = strings.TrimPrefix(path, "loot_tables/")
	path = strings.TrimSuffix(path, ".json")
	if strings.ContainsRune(path, ':') {
		return path
	}
	return "minecraft:" + path
}
