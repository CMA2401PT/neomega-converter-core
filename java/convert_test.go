package java

import (
	"strings"
	"testing"

	"celler"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter-core"
)

func TestConvertChunk(t *testing.T) {
	chunk := cellerjava.NewChunk(celler.ChunkPos{X: 1, Z: -2}, 4189)
	blocks := layer_storage.MakePaletteStorage(ir.IRBlock("minecraft:stone []"))
	blocks.Set(layer_storage.At(0, 0, 0), ir.IRBlock(`minecraft:oak_stairs ["facing"="east","half"="bottom","shape"="straight","waterlogged"="true"]`))
	blocks.Set(layer_storage.At(1, 0, 0), ir.IRBlock("example:missing []"))
	biomes := new([64]string)
	for index := range biomes {
		biomes[index] = "minecraft:plains"
	}
	biomes[1] = "minecraft:beach"
	if err := chunk.SetSection(&cellerjava.Section{Y: 0, Blocks: blocks, Biomes: biomes}); err != nil {
		t.Fatal(err)
	}

	got, err := ConvertChunk(celler.ChunkPos{X: 1, Z: -2}, 0, chunk)
	if err != nil {
		t.Fatal(err)
	}
	if got.Pos != (celler.ChunkPos{X: 1, Z: -2}) || got.Dimension != 0 {
		t.Fatalf("metadata: %+v", got)
	}
	section := got.Sections[0]
	if section == nil {
		t.Fatal("missing section")
	}
	if len(section.Layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(section.Layers))
	}
	base, water := section.Layers[0], section.Layers[1]
	if base.Get(layer_storage.At(2, 2, 2)).State != "minecraft:stone []" {
		t.Fatalf("stone: %v", base.Get(layer_storage.At(2, 2, 2)))
	}
	stairs := base.Get(layer_storage.At(0, 0, 0))
	if strings.Contains(string(stairs.State), "waterlogged") {
		t.Fatalf("waterlogged leaked into base: %s", stairs.State)
	}
	if stairs.Version != bedrockBlockVersion {
		t.Fatalf("version = %d", stairs.Version)
	}
	if water.Get(layer_storage.At(0, 0, 0)).State.NameWithNameSpace() != "minecraft:water" {
		t.Fatalf("water layer: %v", water.Get(layer_storage.At(0, 0, 0)))
	}
	if base.Get(layer_storage.At(1, 0, 0)).State.NameWithNameSpace() != "minecraft:air" {
		t.Fatalf("unknown not air: %v", base.Get(layer_storage.At(1, 0, 0)))
	}

	if len(got.Diagnostics) != 1 {
		t.Fatalf("diagnostics: %+v", got.Diagnostics)
	}
	diagnostic := got.Diagnostics[0]
	if diagnostic.Kind != converter.UnknownBlock || diagnostic.State != "example:missing []" || diagnostic.Count != 1 {
		t.Fatalf("diagnostic: %+v", diagnostic)
	}

	if section.Biomes.Get(layer_storage.At(4, 0, 0)) != 16 || section.Biomes.Get(layer_storage.At(7, 3, 3)) != 16 {
		t.Fatalf("biome copy: %d", section.Biomes.Get(layer_storage.At(4, 0, 0)))
	}
	if section.Biomes.Get(layer_storage.At(0, 0, 0)) != 1 {
		t.Fatalf("default biome: %d", section.Biomes.Get(layer_storage.At(0, 0, 0)))
	}
}

func TestConvertChunkSkipsLightOnlySection(t *testing.T) {
	chunk := cellerjava.NewChunk(celler.ChunkPos{}, 4189)
	if err := chunk.SetSection(&cellerjava.Section{Y: 0}); err != nil {
		t.Fatal(err)
	}
	got, err := ConvertChunk(celler.ChunkPos{}, 0, chunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sections) != 0 || len(got.Diagnostics) != 0 {
		t.Fatalf("sections=%v diagnostics=%v", got.Sections, got.Diagnostics)
	}
}
