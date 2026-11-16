package worldinput

import (
	"celler"
	"celler/bedrock"
	"celler/layer_storage"
	"context"
	nbt "neomega-nbt/presets/v1"
	"strconv"
	"testing"
)

// 自动找地面的出生点和跨维度回退共用最高非空气 section，空气顶层不能抬高相机。
func TestBedrockAutomaticSpawnHeight(t *testing.T) {
	path := t.TempDir()
	world, err := bedrock.Open(path, bedrock.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for dimension := int32(0); dimension < 3; dimension++ {
		chunk := bedrock.NewChunk(celler.ChunkPos{}, dimension)
		for _, y := range []int32{2, 4} {
			layer := bedrock.NewLayer(bedrock.Block{State: "minecraft:air []", Version: 18168865})
			if y == 2 {
				layer.Blocks.Set(layer_storage.At(0, 0, 0), bedrock.Block{State: "minecraft:stone []", Version: 18168865})
			}
			if err := chunk.SetSection(&bedrock.Section{Y: y, Version: 9, Layers: []*bedrock.Layer{layer}}); err != nil {
				t.Fatal(err)
			}
		}
		if err := world.WriteChunk(chunk, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := world.WriteLevel(bedrock.Level{Version: 10, NBT: nbt.Document{Value: nbt.Compound{"SpawnX": int32(3), "SpawnY": int32(32767), "SpawnZ": int32(5)}}}); err != nil {
		t.Fatal(err)
	}
	if err := world.Close(); err != nil {
		t.Fatal(err)
	}
	for dimension := 0; dimension < 3; dimension++ {
		source, err := Open(context.Background(), Options{Path: path, Dimension: strconv.Itoa(dimension)})
		if err != nil {
			t.Fatal(err)
		}
		if source.Info.Position != ([3]float64{3, 56, 5}) {
			t.Fatal(source.Info.Position)
		}
		source.Close()
	}
}
