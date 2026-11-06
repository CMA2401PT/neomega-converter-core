package export_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"celler"
	"celler/bedrock"
	java "celler/java"
	"celler/layer_storage"
	forward "neomega-converter-core/be2je/export"
	"neomega-converter-core/javaout"
	reverse "neomega-converter-core/je2be/export"
	nbt "neomega-nbt/presets/v1"
)

// 跨维度、负坐标和 region 边界比较单任务与并行导出的完整 Java NBT，再反向比较基岩数据。
func TestParallelWorldConversionsPreserveContents(t *testing.T) {
	source, err := bedrock.Open(t.TempDir(), bedrock.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	dimensions := []int32{0, 1, 2}
	positions := []celler.ChunkPos{{X: -2, Z: -1}, {X: 0}, {X: 31}, {X: 32}}
	for _, dim := range dimensions {
		for index, pos := range positions {
			chunk := bedrock.NewChunk(pos, dim)
			solid := bedrock.NewLayer(bedrock.Block{State: "minecraft:stone []", Version: 18168865})
			water := bedrock.NewLayer(bedrock.Block{State: "minecraft:air []", Version: 18168865})
			water.Blocks.Set(layer_storage.At(1, 2, 3), bedrock.Block{State: `minecraft:water ["liquid_depth"=0]`, Version: 18168865})
			if err := chunk.SetSection(&bedrock.Section{Y: 0, Version: 9, Layers: []*bedrock.Layer{solid, water}}); err != nil {
				t.Fatal(err)
			}
			if err := source.WriteChunk(chunk, nil); err != nil {
				t.Fatal(err)
			}
			id := int64(dim*16 + int32(index) + 1)
			actors := []bedrock.Actor{{ID: [8]byte{byte(id)}, NBT: nbt.Document{Value: nbt.Compound{
				"identifier": "minecraft:pig", "UniqueID": id, "Pos": []float32{float32(pos.X*16) + 0.5, 4, float32(pos.Z*16) + 0.5},
			}}}}
			if err := source.WriteChunkActors(dim, pos, actors); err != nil {
				t.Fatal(err)
			}
		}
	}
	var javaWorlds [2]*java.World
	var forwardReports [2]forward.Report
	for i, workers := range []int{1, 4} {
		world, err := java.OpenDirectory(t.TempDir(), java.Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer world.Close()
		javaWorlds[i] = world
		forwardReports[i], err = forward.ConvertWorld(source, world, forward.ConvertOptions{Dimensions: dimensions, Workers: workers})
		if err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(forwardReports[0], forwardReports[1]) {
		t.Fatal(forwardReports)
	}
	for _, dim := range dimensions {
		javaDim, _ := javaout.Dimension(dim)
		for _, pos := range positions {
			a, err := javaWorlds[0].ReadChunk(javaDim, pos)
			if err != nil {
				t.Fatal(err)
			}
			b, err := javaWorlds[1].ReadChunk(javaDim, pos)
			if err != nil || !reflect.DeepEqual(a.NBT, b.NBT) {
				t.Fatal("Java NBT", dim, pos, err)
			}
			aa, err := javaWorlds[0].ReadChunkEntities(javaDim, pos)
			if err != nil {
				t.Fatal(err)
			}
			bb, err := javaWorlds[1].ReadChunkEntities(javaDim, pos)
			if err != nil || !reflect.DeepEqual(aa, bb) {
				t.Fatal("Java entities", dim, pos, err)
			}
		}
	}
	var targets [2]*bedrock.World
	var reverseReports [2]reverse.Report
	for i, workers := range []int{1, 4} {
		target, err := bedrock.Open(t.TempDir(), bedrock.Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer target.Close()
		targets[i] = target
		reverseReports[i], err = reverse.ConvertWorld(javaWorlds[i], target, reverse.ConvertOptions{Dimensions: dimensions, Workers: workers, NewWorld: true})
		if err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(reverseReports[0], reverseReports[1]) {
		t.Fatal(reverseReports)
	}
	for _, dim := range dimensions {
		for _, pos := range positions {
			a, err := targets[0].ReadChunk(dim, pos)
			if err != nil {
				t.Fatal(err)
			}
			b, err := targets[1].ReadChunk(dim, pos)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Records) != len(b.Records) {
				t.Fatal("record count")
			}
			as, err := a.Section(0)
			if err != nil {
				t.Fatal(err)
			}
			bs, err := b.Section(0)
			if err != nil || len(as.Layers) != len(bs.Layers) {
				t.Fatal("layers", err)
			}
			for layer := range as.Layers {
				for x := range int32(16) {
					for y := range int32(16) {
						for z := range int32(16) {
							at := layer_storage.At(x, y, z)
							if as.Layers[layer].Blocks.Get(at) != bs.Layers[layer].Blocks.Get(at) {
								t.Fatal("block", dim, pos, at)
							}
						}
					}
				}
			}
			ab, err := a.Biomes3D()
			if err != nil {
				t.Fatal(err)
			}
			bb, err := b.Biomes3D()
			if err != nil || !reflect.DeepEqual(ab, bb) {
				t.Fatal("biomes", err)
			}
			aa, err := targets[0].ReadChunkActors(dim, pos)
			if err != nil {
				t.Fatal(err)
			}
			ba, err := targets[1].ReadChunkActors(dim, pos)
			if err != nil || !reflect.DeepEqual(aa, ba) {
				t.Fatal("actors", err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = forward.ConvertWorld(source, javaWorlds[0], forward.ConvertOptions{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	_, err = reverse.ConvertWorld(javaWorlds[0], targets[0], reverse.ConvertOptions{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
