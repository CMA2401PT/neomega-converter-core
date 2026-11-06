package structure

import (
	"encoding/json"
	nbt "neomega-nbt/presets/v1"
	"os"
	"path/filepath"
)

// writeHeightPack 为超高 Java 结构提供与区块高度一致的维度定义，游戏加载时不会截断。
func writeHeightPack(dir string, level *nbt.Document, maxY int32) error {
	pack := filepath.Join(dir, "datapacks", "structure-height")
	directory := filepath.Join(pack, "data", "neomega", "dimension_type")
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"结构文件的主世界高度"}}`), 0644); err != nil {
		return err
	}
	dimension := map[string]any{
		"ambient_light": 0, "bed_works": true, "coordinate_scale": 1, "effects": "minecraft:overworld",
		"has_ceiling": false, "has_raids": true, "has_skylight": true, "height": maxY + 64, "logical_height": maxY + 64, "min_y": -64,
		"infiniburn": "#minecraft:infiniburn_overworld", "monster_spawn_block_light_limit": 0,
		"monster_spawn_light_level": map[string]any{"type": "minecraft:uniform", "min_inclusive": 0, "max_inclusive": 7},
		"natural":                   true, "piglin_safe": false, "respawn_anchor_works": false, "ultrawarm": false,
	}
	data, err := json.Marshal(dimension)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(directory, "structure.json"), data, 0644); err != nil {
		return err
	}
	root := level.Value["Data"].(nbt.Compound)
	root["DataPacks"].(nbt.Compound)["Enabled"] = []string{"vanilla", "file/structure-height"}
	root["WorldGenSettings"].(nbt.Compound)["dimensions"].(nbt.Compound)["minecraft:overworld"].(nbt.Compound)["type"] = "neomega:structure"
	return nil
}
