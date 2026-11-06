package structure

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"celler"
	"celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	nbt "neomega-nbt/presets/v1"
)

// 使用显式字段顺序，让方块数组先于尺寸和调色板出现，覆盖真实文件里的乱序字段。
func TestStreamSchemArrayBeforeMetadataAndExtendedHeight(t *testing.T) {
	root := struct {
		BlockData     any
		Width         int16
		Height        int16
		Length        int16
		Version       int32
		Palette       map[string]int32
		BlockEntities []nbt.Compound
	}{
		BlockData: nbt.Array(append([]byte{1}, append(make([]byte, 383), 1)...)), Width: 1, Height: 385, Length: 1, Version: 2,
		Palette:       map[string]int32{"minecraft:air": 0, "minecraft:stone": 1},
		BlockEntities: []nbt.Compound{{"Pos": nbt.Array([]int32{0, 0, 0}), "Id": "minecraft:chest", "Items": []nbt.Compound{}, "CustomName": "kept", "UUID": nbt.Array([]int32{1, 2, 3, 4}), "list": []int32{1, 2, 3, 4}}},
	}
	data, err := nbt.EncodeToNBTBytes(root, nbt.OptEncodeToNBT{Encoding: nbt.EncodingBigEndian})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "tower.schem")
	if err = os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ConvertFile(context.Background(), file, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Directory != WorldDirectory(file) || result.Edition != "java" {
		t.Fatal(result)
	}
	if got := readBlock(t, result.Directory, 0, -64, 0); got != "minecraft:stone []" {
		t.Fatal(got)
	}
	if got := readBlock(t, result.Directory, 0, 320, 0); got != "minecraft:stone []" {
		t.Fatal(got)
	}
	if _, err = os.Stat(filepath.Join(result.Directory, "datapacks", "structure-height", "pack.mcmeta")); err != nil {
		t.Fatal(err)
	}
	world := openWorld(t, result.Directory)
	defer world.Close()
	chunk, err := world.ReadChunk(cellerjava.Overworld, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	tiles, _, err := chunk.BlockEntities()
	if err != nil || len(tiles) != 1 || tiles[0].NBT["CustomName"] != "kept" || tiles[0].Pos.Y != -64 {
		t.Fatal(tiles, err)
	}
	if uuid, ok := tiles[0].NBT["UUID"].([4]int32); !ok || uuid != [4]int32{1, 2, 3, 4} {
		t.Fatal("IntArray 类型丢失", tiles[0].NBT)
	}
	if _, ok := tiles[0].NBT["list"].([]int32); !ok {
		t.Fatal("Int List 类型丢失", tiles[0].NBT)
	}
	// 已有结果不可因再次转换被覆盖。
	if _, err = ConvertFile(context.Background(), file, ""); err == nil {
		t.Fatal("existing world overwritten")
	}
}

func TestStreamNativeNBTAndLegacy(t *testing.T) {
	for _, kind := range []string{"nbt", "schematic"} {
		t.Run(kind, func(t *testing.T) {
			var root map[string]any
			if kind == "nbt" {
				root = map[string]any{"size": []int32{17, 1, 1}, "palette": []nbt.Compound{{"Name": "minecraft:stone"}}, "blocks": []nbt.Compound{{"pos": []int32{16, 0, 0}, "state": int32(0)}}, "entities": []nbt.Compound{{"pos": []float64{16.5, 1, 0}, "blockPos": []int32{16, 1, 0}, "nbt": nbt.Compound{"id": "minecraft:armor_stand"}}}}
			} else {
				blocks := make([]byte, 17)
				blocks[16] = 1
				root = map[string]any{"Width": int16(17), "Height": int16(1), "Length": int16(1), "Materials": "Alpha", "Blocks": nbt.Array(blocks), "Data": nbt.Array(make([]byte, 17))}
			}
			file := writeNBT(t, "sample."+kind, root, nbt.EncodingBigEndian)
			result, err := ConvertFile(context.Background(), file, "")
			if err != nil {
				t.Fatal(err)
			}
			if got := readBlock(t, result.Directory, 16, 0, 0); got != "minecraft:stone []" {
				t.Fatal(got)
			}
			if kind == "nbt" {
				world := openWorld(t, result.Directory)
				defer world.Close()
				entities, err := world.ReadChunkEntities(cellerjava.Overworld, celler.ChunkPos{X: 1})
				if err != nil || len(entities) != 1 || entities[0]["id"] != "minecraft:armor_stand" {
					t.Fatal(entities, err)
				}
			}
		})
	}
}

func TestStreamMCStructureKeepsBothLayers(t *testing.T) {
	root := map[string]any{"format_version": int32(1), "structure_world_origin": []int32{-1, 2, -1}, "size": []int32{1, 1, 1}, "structure": nbt.Compound{
		"block_indices": []any{[]int32{0}, []int32{1}},
		"palette": nbt.Compound{"default": nbt.Compound{"block_palette": []nbt.Compound{
			{"name": "minecraft:stone", "states": nbt.Compound{}, "version": int32(18168865)},
			{"name": "minecraft:water", "states": nbt.Compound{"liquid_depth": int32(0)}, "version": int32(18168865)},
		}}}}}
	file := writeNBT(t, "water.mcstructure", root, nbt.EncodingLittleEndian)
	result, err := ConvertFile(context.Background(), file, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Edition != "bedrock" {
		t.Fatal(result)
	}
	world, err := bedrock.Open(result.Directory, bedrock.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer world.Close()
	chunk, err := world.ReadChunk(0, celler.ChunkPos{X: -1, Z: -1})
	if err != nil {
		t.Fatal(err)
	}
	section, err := chunk.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	if section.Layers[0].Blocks.Get(layer_storage.At(15, 2, 15)).State.NameWithNameSpace() != "minecraft:stone" || section.Layers[1].Blocks.Get(layer_storage.At(15, 2, 15)).State.NameWithNameSpace() != "minecraft:water" {
		t.Fatal(section)
	}
}

func TestStreamFailureLeavesNoWorld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.schem")
	if err := os.WriteFile(path, []byte{10, 0, 0, 7}, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ConvertFile(context.Background(), path, ""); err == nil {
		t.Fatal("truncated input accepted")
	}
	if _, err := os.Stat(WorldDirectory(path)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// 有效文件仍须传播取消，不能发布输出目录。
	file := writeNBT(t, "cancel.schem", map[string]any{"Version": int32(2), "Width": int16(1), "Height": int16(1), "Length": int16(1), "Palette": map[string]any{"minecraft:stone": int32(0)}, "BlockData": nbt.Array([]byte{0})}, nbt.EncodingBigEndian)
	if _, err := ConvertFile(ctx, file, ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStreamSchemV3VarintCrossesBuffer(t *testing.T) {
	data := make([]byte, 65535)
	data = append(data, 0xac, 0x02)
	root := map[string]any{"Schematic": map[string]any{"Version": int32(3), "Width": int16(256), "Height": int16(1), "Length": int16(256), "Blocks": map[string]any{"Palette": map[string]any{"minecraft:air": int32(0), "minecraft:stone": int32(300)}, "Data": nbt.Array(data)}}}
	file := writeNBT(t, "wide.schem", root, nbt.EncodingBigEndian)
	result, err := ConvertFile(context.Background(), file, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := readBlock(t, result.Directory, 255, 0, 255); got != "minecraft:stone []" {
		t.Fatal(got)
	}
}

func TestStreamLitematicNegativeSizeAndCrossWord(t *testing.T) {
	palette := []nbt.Compound{{"Name": "minecraft:air"}, {"Name": "minecraft:stone"}, {"Name": "minecraft:dirt"}, {"Name": "minecraft:glass"}, {"Name": "minecraft:gold_block"}}
	// 第 21 格从 bit 63 开始，占用两个 long；负尺寸的最小 X 是 -23。
	root := map[string]any{"Version": int32(6), "Regions": map[string]any{"main": map[string]any{
		"Position": map[string]any{"x": int32(0), "y": int32(0), "z": int32(0)}, "Size": map[string]any{"x": int32(-24), "y": int32(1), "z": int32(1)},
		"BlockStatePalette": palette, "BlockStates": nbt.Array([]int64{0, 2}),
		"TileEntities": []nbt.Compound{{"x": int32(21), "y": int32(0), "z": int32(0), "id": "minecraft:chest"}},
	}}}
	file := writeNBT(t, "cross.litematic", root, nbt.EncodingBigEndian)
	result, err := ConvertFile(context.Background(), file, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := readBlock(t, result.Directory, -2, 0, 0); got != "minecraft:gold_block []" {
		t.Fatal(got)
	}
}

func TestStreamingSequenceDoesNotRetainArray(t *testing.T) {
	s := &numberStream{dir: t.TempDir(), varints: true}
	total := 64 * 1024 * 64
	calls := 0
	handled, err := s.ConsumeByteArray(total, func(buffer []byte) error { calls++; clear(buffer); return nil })
	if err != nil || !handled || calls != 64 || s.count != int64(total) {
		t.Fatal(handled, err, calls, s.count)
	}
	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 100 {
		t.Fatalf("single run expanded to %d bytes", info.Size())
	}
	runs := 0
	if err = s.eachRun(func(value, count uint64) error {
		runs++
		if value != 0 || count != uint64(total) {
			t.Fatal(value, count)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatal(runs)
	}
}

func TestSchematicSpongeTileWrapper(t *testing.T) {
	root := map[string]any{"Width": int16(1), "Height": int16(1), "Length": int16(1), "Blocks": nbt.Array([]byte{23}), "Data": nbt.Array([]byte{2}), "TileEntities": []nbt.Compound{{"Id": "minecraft:dispenser", "Pos": nbt.Array([]int32{0, 0, 0}), "CustomName": "kept"}}}
	file := writeNBT(t, "mixed.schematic", root, nbt.EncodingBigEndian)
	result, err := ConvertFile(context.Background(), file, "")
	if err != nil {
		t.Fatal(err)
	}
	world := openWorld(t, result.Directory)
	defer world.Close()
	chunk, err := world.ReadChunk(cellerjava.Overworld, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	tiles, _, err := chunk.BlockEntities()
	if err != nil || len(tiles) != 1 || tiles[0].NBT["CustomName"] != "kept" || tiles[0].NBT["id"] != "minecraft:dispenser" {
		t.Fatal(tiles, err)
	}
}

func TestLitematicOverlapClearsBlocksAndTiles(t *testing.T) {
	region := func(state string, tiles []nbt.Compound) map[string]any {
		return map[string]any{"Position": map[string]int32{"x": 0, "y": 0, "z": 0}, "Size": map[string]int32{"x": 2, "y": 1, "z": 1}, "BlockStatePalette": []nbt.Compound{{"Name": state}}, "BlockStates": nbt.Array([]int64{0}), "TileEntities": tiles}
	}
	root := map[string]any{"Version": int32(6), "Regions": map[string]any{"a": region("minecraft:chest", []nbt.Compound{{"x": int32(0), "y": int32(0), "z": int32(0), "id": "minecraft:chest"}}), "b": region("minecraft:air", nil)}}
	result, err := ConvertFile(context.Background(), writeNBT(t, "overlap.litematic", root, nbt.EncodingBigEndian), "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Chunks != 0 {
		t.Fatalf("被覆盖的方块或实体仍写出了 %d 个区块", result.Chunks)
	}
}

func TestGzipChecksumFailureLeavesNoWorld(t *testing.T) {
	root := map[string]any{"Version": int32(2), "Width": int16(1), "Height": int16(1), "Length": int16(1), "Palette": map[string]int32{"minecraft:stone": 0}, "BlockData": nbt.Array([]byte{0})}
	raw, err := nbt.EncodeToNBTBytes(root, nbt.OptEncodeToNBT{Encoding: nbt.EncodingBigEndian})
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	zipped := gzip.NewWriter(&compressed)
	if _, err = zipped.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err = zipped.Close(); err != nil {
		t.Fatal(err)
	}
	data := compressed.Bytes()
	data[len(data)-8] ^= 1
	file := filepath.Join(t.TempDir(), "corrupt.schem")
	if err = os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = ConvertFile(context.Background(), file, ""); !errors.Is(err, gzip.ErrChecksum) {
		t.Fatal(err)
	}
	if _, err = os.Stat(WorldDirectory(file)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestMCWorldExtractsNestedAndRejectsTraversal(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "pack.mcworld")
			f, err := os.Create(file)
			if err != nil {
				t.Fatal(err)
			}
			zipped := zip.NewWriter(f)
			names := []string{"world/level.dat", "world/db/CURRENT"}
			if bad {
				names = append(names, "world/../escape")
			}
			for _, name := range names {
				writer, err := zipped.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = writer.Write([]byte("data")); err != nil {
					t.Fatal(err)
				}
			}
			if err = zipped.Close(); err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
			result, err := ConvertFile(context.Background(), file, "")
			if bad {
				if err == nil {
					t.Fatal("越界路径被接受")
				}
				if _, err = os.Stat(WorldDirectory(file)); !os.IsNotExist(err) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(result.Directory, "db", "CURRENT"))
			if err != nil || string(data) != "data" || result.Edition != "bedrock" {
				t.Fatal(result, string(data), err)
			}
		})
	}
}
