package actors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEntityDefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bedrock_entities.json")
	if err := os.WriteFile(path, []byte(`{"zombie":{"spawn":["minecraft:zombie_adult"],"health":20}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	defer delete(EntityDefs, "minecraft:zombie")
	if err := LoadEntityDefs(path); err != nil {
		t.Fatal(err)
	}
	def := EntityDefs["minecraft:zombie"]
	if def.Health != 20 || len(def.Spawn) != 1 || def.Spawn[0] != "minecraft:zombie_adult" {
		t.Fatalf("definition: %#v", def)
	}
}
