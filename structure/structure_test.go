package structure

import (
	"celler"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"os"
	"path/filepath"
	"testing"
)

func writeNBT(t *testing.T, name string, root map[string]any, encoding nbt.Encoding) string {
	t.Helper()
	raw, err := nbt.EncodeToNBTBytes(root, nbt.OptEncodeToNBT{Encoding: encoding})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err = os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func openWorld(t *testing.T, dir string) *cellerjava.World {
	t.Helper()
	world, err := cellerjava.OpenDirectory(dir, cellerjava.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	return world
}

func readBlock(t *testing.T, dir string, x, y, z int32) ir.IRBlock {
	t.Helper()
	world := openWorld(t, dir)
	defer world.Close()
	chunk, err := world.ReadChunk(cellerjava.Overworld, celler.ChunkPos{X: x >> 4, Z: z >> 4})
	if err != nil {
		t.Fatal(err)
	}
	section, err := chunk.Section(y >> 4)
	if err != nil {
		t.Fatal(err)
	}
	return section.Blocks.Get(layer_storage.At(x, y, z))
}
