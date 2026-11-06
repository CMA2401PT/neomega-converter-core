package structure

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"celler"
	cellerjava "celler/java"
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
