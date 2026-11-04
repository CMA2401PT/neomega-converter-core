package bedrock

import (
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
)

func biomeChunk(t *testing.T, ys ...int32) *cellerbedrock.Chunk {
	t.Helper()
	chunk := cellerbedrock.NewChunk(celler.ChunkPos{}, 0)
	for _, y := range ys {
		layer := cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:stone []", Version: 18168865})
		if err := chunk.SetSection(&cellerbedrock.Section{Y: y, Version: 9, Layers: []*cellerbedrock.Layer{layer}}); err != nil {
			t.Fatal(err)
		}
	}
	return chunk
}

func TestBiomes3D(t *testing.T) {
	chunk := biomeChunk(t, -4, -3)
	first := layer_storage.MakePaletteStorage(int32(1)) // 1 = plains
	first.Set(layer_storage.At(4, 0, 0), int32(16))     // 群系 16 = beach
	first.Set(layer_storage.At(0, 4, 0), int32(9999))   // 未知编号回退 plains
	data := &cellerbedrock.Biomes3D{Sections: []cellerbedrock.BiomeSection{
		{Values: first},
		{}, // 0xFF：继承上一节
	}}
	if err := chunk.SetBiomes3D(data); err != nil {
		t.Fatal(err)
	}
	got, err := ConvertChunk(chunk)
	if err != nil {
		t.Fatal(err)
	}
	for _, y := range []int32{-4, -3} {
		biomes := got.Sections[y].Biomes
		// Java 每 section 的 64 个值按 y<<4|z<<2|x 排列。
		if biomes[1] != "minecraft:beach" {
			t.Fatalf("section %d x=1: %v", y, biomes[1])
		}
		if biomes[1<<4] != "minecraft:plains" {
			t.Fatalf("section %d unknown fallback: %v", y, biomes[1<<4])
		}
		if biomes[0] != "minecraft:plains" {
			t.Fatalf("section %d default: %v", y, biomes[0])
		}
	}
}

func TestBiomes2D(t *testing.T) {
	chunk := biomeChunk(t, -4, 0)
	var data cellerbedrock.Biomes2D
	data.Values[4] = 16 // 列 x=4、z=0，对 4×4 单元的 x=1 取样
	chunk.SetBiomes2D(data)
	got, err := ConvertChunk(chunk)
	if err != nil {
		t.Fatal(err)
	}
	for _, y := range []int32{-4, 0} {
		biomes := got.Sections[y].Biomes
		// 其余列为 0 = ocean（零值的测试数据），不是默认 plains。
		if biomes[1] != "minecraft:beach" || biomes[0] != "minecraft:ocean" {
			t.Fatalf("section %d: %v", y, biomes)
		}
	}
}

func TestBiomesMissingUsePlains(t *testing.T) {
	got, err := ConvertChunk(biomeChunk(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	biomes := got.Sections[0].Biomes
	if biomes == nil {
		t.Fatal("missing biomes")
	}
	for index, name := range biomes {
		if name != "minecraft:plains" {
			t.Fatalf("biome %d = %q", index, name)
		}
	}
}
