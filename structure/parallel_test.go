package structure

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"celler"
	"celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	nbt "neomega-nbt/presets/v1"
)

// 方块、方块实体和实体分布在不同片区，检查任务完成顺序不影响全部 NBT。
func TestParallelWorldMatchesSerialAndCancels(t *testing.T) {
	var blocks, entities []nbt.Compound
	for x := int32(0); x <= 512; x += 128 {
		for z := int32(0); z <= 128; z += 128 {
			blocks = append(blocks, nbt.Compound{
				"pos": []int32{x, 0, z}, "state": int32(0),
				"nbt": nbt.Compound{"id": "minecraft:chest", "Items": []nbt.Compound{}, "CustomName": "kept"},
			})
			entities = append(entities, nbt.Compound{
				"pos": []float64{float64(x) + 0.5, 1, float64(z) + 0.5}, "blockPos": []int32{x, 1, z},
				"nbt": nbt.Compound{"id": "minecraft:armor_stand", "UUID": nbt.Array([]int32{x, z, 1, 2})},
			})
		}
	}
	file := writeNBT(t, "multiple.nbt", map[string]any{
		"size": []int32{513, 2, 129}, "palette": []nbt.Compound{{"Name": "minecraft:chest"}},
		"blocks": blocks, "entities": entities,
	}, nbt.EncodingBigEndian)
	output := t.TempDir()
	serial, err := ConvertFile(context.Background(), file, filepath.Join(output, "serial"), Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	parallel, err := ConvertFile(context.Background(), file, filepath.Join(output, "parallel"), Options{Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	left, right := openWorld(t, serial.Directory), openWorld(t, parallel.Directory)
	defer left.Close()
	defer right.Close()
	for _, kind := range []cellerjava.RegionKind{cellerjava.Terrain, cellerjava.Entities} {
		count := 0
		err := left.Scan(cellerjava.Overworld, kind, func(pos celler.ChunkPos) error {
			l, err := left.ReadRaw(cellerjava.Overworld, kind, pos)
			if err != nil {
				return err
			}
			r, err := right.ReadRaw(cellerjava.Overworld, kind, pos)
			if err != nil {
				return err
			}
			ld, err := l.Decode()
			if err != nil {
				return err
			}
			rd, err := r.Decode()
			if err != nil {
				return err
			}
			ln, err := nbt.DecodeDocument(ld, nbt.EncodingBigEndian)
			if err != nil {
				return err
			}
			rn, err := nbt.DecodeDocument(rd, nbt.EncodingBigEndian)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(ln, rn) {
				t.Errorf("%s %v 的 NBT 不一致", kind, pos)
			}
			count++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		other := 0
		if err := right.Scan(cellerjava.Overworld, kind, func(celler.ChunkPos) error { other++; return nil }); err != nil {
			t.Fatal(err)
		}
		if count != 10 || other != count {
			t.Fatal(kind, count, other)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelled := filepath.Join(output, "cancelled")
	_, err = ConvertFile(ctx, file, cancelled, Options{Workers: 4, Progress: func(p Progress) {
		if p.Stage == "writing" && p.Completed == 0 {
			cancel()
		}
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(cancelled); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("取消后发布了不完整世界", err)
	}
	paths, err := filepath.Glob(filepath.Join(output, ".structure-*"))
	if err != nil || len(paths) != 0 {
		t.Fatal("取消后临时目录未释放", paths, err)
	}
}

// 双层结构跨多个片区；比较原生方块版本、方块实体和普通实体，并检查并发编号唯一。
func TestParallelMCStructurePreservesLayersAndActors(t *testing.T) {
	const width, length = 257, 129
	primary, secondary := make([]int32, width*length), make([]int32, width*length)
	for i := range primary {
		primary[i], secondary[i] = -1, -1
	}
	var entities []nbt.Compound
	tiles := map[string]any{}
	var positions []celler.BlockPos
	for x := int32(0); x < width; x += 128 {
		for z := int32(0); z < length; z += 128 {
			index := int(x)*length + int(z)
			primary[index], secondary[index] = 0, 1
			positions = append(positions, celler.BlockPos{X: x - 129, Y: -4, Z: z - 129})
			entities = append(entities, nbt.Compound{"identifier": "minecraft:pig", "Pos": []float32{float32(x) + 0.5, 1, float32(z) + 0.5}})
			tiles[strconv.Itoa(index)] = nbt.Compound{"block_entity_data": nbt.Compound{"id": "Chest", "CustomName": "kept", "Items": []nbt.Compound{}}}
		}
	}
	root := map[string]any{
		"format_version": int32(1), "structure_world_origin": []int32{-129, -4, -129}, "size": []int32{width, 1, length},
		"structure": nbt.Compound{
			"block_indices": []any{primary, secondary}, "entities": entities,
			"palette": nbt.Compound{"default": nbt.Compound{
				"block_palette": []nbt.Compound{
					{"name": "minecraft:chest", "states": nbt.Compound{"facing_direction": int32(2)}, "version": int32(18168865)},
					{"name": "minecraft:water", "states": nbt.Compound{"liquid_depth": int32(0)}, "version": int32(18168865)},
				}, "block_position_data": tiles,
			}},
		},
	}
	file := writeNBT(t, "multiple.mcstructure", root, nbt.EncodingLittleEndian)
	output := t.TempDir()
	var worlds [2]*bedrock.World
	for i, workers := range []int{1, 4} {
		result, err := ConvertFile(context.Background(), file, filepath.Join(output, strconv.Itoa(workers)), Options{Workers: workers})
		if err != nil {
			t.Fatal(err)
		}
		world, err := bedrock.Open(result.Directory, bedrock.Options{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer world.Close()
		worlds[i] = world
	}
	seen := map[int64]bool{}
	for _, pos := range positions {
		a, err := worlds[0].ReadChunk(0, pos.Chunk())
		if err != nil {
			t.Fatal(err)
		}
		b, err := worlds[1].ReadChunk(0, pos.Chunk())
		if err != nil {
			t.Fatal(err)
		}
		aa, err := a.Section(pos.Y >> 4)
		if err != nil {
			t.Fatal(err)
		}
		bb, err := b.Section(pos.Y >> 4)
		if err != nil || len(aa.Layers) != 2 || len(bb.Layers) != 2 {
			t.Fatal(err)
		}
		for layer := range 2 {
			av := aa.Layers[layer].Blocks.Get(layer_storage.At(pos.X, pos.Y, pos.Z))
			bv := bb.Layers[layer].Blocks.Get(layer_storage.At(pos.X, pos.Y, pos.Z))
			if av != bv || av.Version != 18168865 {
				t.Fatal(av, bv)
			}
			want := "minecraft:chest"
			if layer == 1 {
				want = "minecraft:water"
			}
			if av.State.NameWithNameSpace() != want {
				t.Fatal(av)
			}
		}
		at, err := a.BlockEntities()
		if err != nil {
			t.Fatal(err)
		}
		bt, err := b.BlockEntities()
		if err != nil || len(at) != 1 || !reflect.DeepEqual(at, bt) || at[0].Value["CustomName"] != "kept" {
			t.Fatal(at, bt, err)
		}
		ae, err := worlds[0].ReadChunkActors(0, pos.Chunk())
		if err != nil {
			t.Fatal(err)
		}
		be, err := worlds[1].ReadChunkActors(0, pos.Chunk())
		if err != nil || len(ae) != 1 || !reflect.DeepEqual(ae, be) {
			t.Fatal(ae, be, err)
		}
		id := ae[0].Value["UniqueID"].(int64)
		if seen[id] {
			t.Fatal("重复实体编号", id)
		}
		seen[id] = true
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelled := filepath.Join(output, "cancelled")
	_, err := ConvertFile(ctx, file, cancelled, Options{Workers: 4, Progress: func(p Progress) {
		if p.Stage == "writing" {
			cancel()
		}
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(cancelled); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(output, ".structure-*"))
	if err != nil || len(paths) != 0 {
		t.Fatal(paths, err)
	}
}
