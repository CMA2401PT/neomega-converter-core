package worldinput

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"celler"
	"celler/java"
	"celler/layer_storage"
	"celler/storage"
	"neomega-blocks/ir"
	"neomega-converter-core/structure"
	nbt "neomega-nbt/presets/v1"
	packfs "neomega-packfs"
)

// 同一维度入口必须在目录和只读容器中得到相同的区块及维度列表。
func TestJavaDimensionsAndPack(t *testing.T) {
	for _, packed := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "pack"}[packed], func(t *testing.T) {
			path := t.TempDir()
			var world *java.World
			var container *packfs.PackFS
			var err error
			if packed {
				path = filepath.Join(path, "world.pack")
				container, err = packfs.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				world, err = java.Open(storage.Pack{PackFS: container}, java.Options{})
			} else {
				world, err = java.OpenDirectory(path, java.Options{})
			}
			if err != nil {
				t.Fatal(err)
			}
			dimensions := []string{".", "DIM-1", "DIM1", "dimensions/demo/moon"}
			for i, dim := range dimensions {
				chunk := java.NewChunk(celler.ChunkPos{X: int32(i)}, 4189)
				if err := chunk.SetSection(&java.Section{Y: 0, Blocks: layer_storage.MakePaletteStorage(ir.IRBlock("minecraft:stone []"))}); err != nil {
					t.Fatal(err)
				}
				if err := world.WriteChunk(java.Dimension(dim), chunk); err != nil {
					t.Fatal(err)
				}
			}
			if err := world.WriteLevel(nbt.Document{Value: nbt.Compound{"Data": nbt.Compound{}}}); err != nil {
				t.Fatal(err)
			}
			if err := world.Close(); err != nil {
				t.Fatal(err)
			}
			if packed {
				if err := container.Close(); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0444); err != nil {
					t.Fatal(err)
				}
			}
			for i, dim := range dimensions {
				source, err := Open(context.Background(), Options{Path: path, Dimension: dim})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(source.Info.Dimensions, dimensions) || source.Info.Dimension != dim {
					t.Fatal(source.Info)
				}
				chunks, err := source.Java.Chunks(context.Background())
				if err != nil || !reflect.DeepEqual(chunks, []celler.ChunkPos{{X: int32(i)}}) {
					t.Fatal(chunks, err)
				}
				records, err := source.Java.Read(context.Background(), chunks[0])
				if err != nil || records[0] == nil {
					t.Fatal(records, err)
				}
				if err := source.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// 打开结构沿用转换器的预算和进度；再次打开直接复用原生世界。
func TestStructureConversionAndReuse(t *testing.T) {
	root := nbt.Compound{
		"size":    []int32{1, 1, 1},
		"palette": []nbt.Compound{{"Name": "minecraft:stone"}},
		"blocks":  []nbt.Compound{{"pos": []int32{0, 0, 0}, "state": int32(0)}},
	}
	raw, err := nbt.EncodeToNBTBytes(root, nbt.OptEncodeToNBT{Encoding: nbt.EncodingBigEndian})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sample.nbt")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	options := Options{Path: path, Structure: structure.Options{MemoryMiB: 32}}
	if _, err := Open(context.Background(), options); err == nil {
		t.Fatal("转换未使用预算")
	}
	progress := 0
	options.Structure = structure.Options{MemoryMiB: 64, Workers: 1, Progress: func(value structure.Progress) { progress++ }}
	source, err := Open(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if source.Info.ReusedStructure || source.Info.ConversionSeconds == nil || progress == 0 || source.Info.Kind != "nbt" {
		t.Fatal(source.Info, progress)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	// 破坏原文件仍能打开已有输出，证明不会重新转换或覆盖。
	if err := os.WriteFile(path, []byte("not nbt"), 0644); err != nil {
		t.Fatal(err)
	}
	progress = 0
	source, err = Open(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if !source.Info.ReusedStructure || source.Info.ConversionSeconds != nil || progress != 0 {
		t.Fatal(source.Info, progress)
	}
}
