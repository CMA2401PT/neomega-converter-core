package actors

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LoadEntityDefs 读取 mcconv 风格的 bedrock_entities.json：
// {"minecraft:zombie": {"spawn": [...], "born": [...], "health": 20}}。
func LoadEntityDefs(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var defs map[string]EntityDef
	if err := json.Unmarshal(data, &defs); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for name, def := range defs {
		if !strings.ContainsRune(name, ':') {
			name = "minecraft:" + name
		}
		EntityDefs[name] = def
	}
	return nil
}
