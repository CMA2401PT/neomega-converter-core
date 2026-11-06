package export

import (
	"errors"
	"io/fs"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter-core"
	nbt "neomega-nbt/presets/v1"
)

func openBedrock(t *testing.T) *cellerbedrock.World {
	t.Helper()
	world, err := cellerbedrock.Open(t.TempDir(), cellerbedrock.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := world.Close(); err != nil {
			t.Error(err)
		}
	})
	return world
}

func openJava(t *testing.T) *cellerjava.World {
	t.Helper()
	world, err := cellerjava.OpenDirectory(t.TempDir(), cellerjava.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := world.Close(); err != nil {
			t.Error(err)
		}
	})
	return world
}

func sourceChunk(t *testing.T, pos celler.ChunkPos, dim int32, ys ...int32) *cellerbedrock.Chunk {
	t.Helper()
	chunk := cellerbedrock.NewChunk(pos, dim)
	for _, y := range ys {
		layer := cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:stone []", Version: 18168865})
		if err := chunk.SetSection(&cellerbedrock.Section{Y: y, Version: 9, Layers: []*cellerbedrock.Layer{layer}}); err != nil {
			t.Fatal(err)
		}
	}
	return chunk
}

func TestConvertWorld(t *testing.T) {
	source, target := openBedrock(t), openJava(t)
	chunk := sourceChunk(t, celler.ChunkPos{}, 0, 0, 20)
	section, err := chunk.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	for index, state := range []ir.IRBlock{"minecraft:chest []", `minecraft:frame ["facing_direction"=3]`, "minecraft:furnace []", "minecraft:not_a_block []"} {
		section.Layers[0].Blocks.Set(layer_storage.At(index, 0, 0), cellerbedrock.Block{State: state.Tidy(), Version: 18168865})
	}
	if err = chunk.SetSection(section); err != nil {
		t.Fatal(err)
	}
	if err = chunk.SetBlockEntities([]nbt.Document{
		{Value: nbt.Compound{"id": "Chest", "x": int32(0), "y": int32(0), "z": int32(0), "Items": []nbt.Compound{}}},
		{Value: nbt.Compound{"id": "ItemFrame", "x": int32(1), "y": int32(0), "z": int32(0)}},
		{Value: nbt.Compound{"id": "Beehive", "x": int32(2), "y": int32(0), "z": int32(0)}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*cellerbedrock.Chunk{chunk, sourceChunk(t, celler.ChunkPos{}, 1, -1, 0, 16), sourceChunk(t, celler.ChunkPos{}, 2, 0)} {
		if err = source.WriteChunk(c, nil); err != nil {
			t.Fatal(err)
		}
	}
	report, err := ConvertWorld(source, target, ConvertOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Chunks[0] != 1 || len(report.Chunks) != 1 || report.DroppedSections[0] != 1 || report.Entities != 1 || report.BlockEntities != 1 {
		t.Fatalf("report=%+v", report)
	}
	if report.Diagnostics[converter.UnknownBlock] != 1 || report.Diagnostics[converter.UnsupportedBlockEntity] != 1 || report.UnsupportedBlockEntities["Beehive"] != 1 || len(report.UnknownStates) != 1 || report.UnknownStates[0].Count != 1 {
		t.Fatalf("diagnostics=%+v", report)
	}
	got, err := target.ReadChunk(cellerjava.Overworld, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := got.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	if blocks.Blocks.Get(layer_storage.At(3, 0, 0)) != "minecraft:air []" {
		t.Fatal("unknown not air")
	}
	entities, issues, err := got.BlockEntities()
	if err != nil || len(issues) != 0 || len(entities) != 1 {
		t.Fatalf("entities=%v issues=%v err=%v", entities, issues, err)
	}
	if _, err = target.ReadRaw(cellerjava.Overworld, cellerjava.Entities, celler.ChunkPos{}); err != nil {
		t.Fatal(err)
	}
	if _, err = target.ReadChunk(cellerjava.Nether, celler.ChunkPos{}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unselected dimension: %v", err)
	}
	all := openJava(t)
	report, err = ConvertWorld(source, all, ConvertOptions{Dimensions: []int32{0, 1, 2}})
	if err != nil || report.Chunks[1] != 1 || report.Chunks[2] != 1 || report.DroppedSections[1] != 2 {
		t.Fatalf("all=%+v err=%v", report, err)
	}
	if _, err = all.ReadRaw(cellerjava.Nether, cellerjava.Entities, celler.ChunkPos{}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("empty entity record: %v", err)
	}
}

func TestConvertWorldStrict(t *testing.T) {
	source := openBedrock(t)
	for x := range int32(3) {
		chunk := sourceChunk(t, celler.ChunkPos{X: x}, 0, 0)
		if x == 1 {
			chunk.Records[string([]byte{cellerbedrock.TagSubChunk, 0})] = []byte{99}
		}
		if err := source.WriteChunk(chunk, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, strict := range []bool{false, true} {
		target := openJava(t)
		report, err := ConvertWorld(source, target, ConvertOptions{Strict: strict})
		if (err != nil) != strict || len(report.FailedChunks) != 1 || report.FailedChunks[0].Pos.X != 1 || report.FailedChunks[0].Err == nil {
			t.Fatalf("strict=%v report=%+v err=%v", strict, report, err)
		}
		want := uint64(2)
		if strict {
			want = 1
		}
		if report.Chunks[0] != want {
			t.Fatal(report)
		}
		if _, err = target.ReadChunk(cellerjava.Overworld, celler.ChunkPos{}); err != nil {
			t.Fatal(err)
		}
		if _, err = target.ReadChunk(cellerjava.Overworld, celler.ChunkPos{X: 1}); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("bad chunk written: %v", err)
		}
		// 另开读句柄，确认包括严格模式提前退出在内的所有返回路径均已 Flush。
		reader, err := cellerjava.Open(target.Files, cellerjava.Options{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		_, err = reader.ReadChunk(cellerjava.Overworld, celler.ChunkPos{})
		closeErr := reader.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("flush: %v %v", err, closeErr)
		}
	}
}

func TestConvertWorldSkipsUnfinished(t *testing.T) {
	source := openBedrock(t)
	chunk := sourceChunk(t, celler.ChunkPos{}, 0, 0)
	chunk.Records[string([]byte{cellerbedrock.TagFinalizedState})] = []byte{1}
	if err := source.WriteChunk(chunk, nil); err != nil {
		t.Fatal(err)
	}
	target := openJava(t)
	report, err := ConvertWorld(source, target, ConvertOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Chunks[0] != 0 || report.Skipped != 1 {
		t.Fatalf("report=%+v", report)
	}
	if _, err := target.ReadChunk(cellerjava.Overworld, celler.ChunkPos{}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unfinished chunk written: %v", err)
	}

	all := openJava(t)
	report, err = ConvertWorld(source, all, ConvertOptions{IncludeUnfinished: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Chunks[0] != 1 || report.Skipped != 0 {
		t.Fatalf("all=%+v", report)
	}
	if _, err := all.ReadChunk(cellerjava.Overworld, celler.ChunkPos{}); err != nil {
		t.Fatalf("included chunk missing: %v", err)
	}
}

func TestFinalizedState(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		want int32
	}{
		{"missing", nil, 2},
		{"byte finished", []byte{2}, 2},
		{"byte unfinished", []byte{1}, 1},
		{"int unfinished", []byte{1, 0, 0, 0}, 1},
		{"unknown length", []byte{1, 2}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := cellerbedrock.NewChunk(celler.ChunkPos{}, 0)
			if test.data != nil {
				chunk.Records[string([]byte{cellerbedrock.TagFinalizedState})] = test.data
			}
			if got := finalizedState(chunk); got != test.want {
				t.Fatalf("finalizedState = %d, want %d", got, test.want)
			}
		})
	}
}

func TestConvertWorldActors(t *testing.T) {
	source, target := openBedrock(t), openJava(t)
	pos := celler.ChunkPos{}
	if err := source.WriteChunk(sourceChunk(t, pos, 0, 0), nil); err != nil {
		t.Fatal(err)
	}
	actorKey := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	value, err := nbt.EncodeDocument(nbt.Document{Value: nbt.Compound{
		"identifier": "minecraft:zombie", "Pos": []float32{0, 65, 0},
	}}, nbt.EncodingLittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	if err = source.DB.Put(append([]byte("actorprefix"), actorKey...), value, nil); err != nil {
		t.Fatal(err)
	}
	digp := append([]byte("digp"), cellerbedrock.ChunkPrefix(pos, 0)...)
	if err = source.DB.Put(digp, actorKey, nil); err != nil {
		t.Fatal(err)
	}
	report, err := ConvertWorld(source, target, ConvertOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Entities != 1 {
		t.Fatalf("report=%+v", report)
	}
	if _, err = target.ReadRaw(cellerjava.Overworld, cellerjava.Entities, pos); err != nil {
		t.Fatalf("entity record: %v", err)
	}
}

func TestConvertWorldActorErrorKeepsTerrain(t *testing.T) {
	source, target := openBedrock(t), openJava(t)
	pos := celler.ChunkPos{}
	if err := source.WriteChunk(sourceChunk(t, pos, 0, 0), nil); err != nil {
		t.Fatal(err)
	}
	// 长度不是 8 的倍数：读取实体失败，但地形必须照常转换。
	digp := append([]byte("digp"), cellerbedrock.ChunkPrefix(pos, 0)...)
	if err := source.DB.Put(digp, []byte{1}, nil); err != nil {
		t.Fatal(err)
	}
	report, err := ConvertWorld(source, target, ConvertOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Chunks[0] != 1 || len(report.FailedChunks) != 0 {
		t.Fatalf("report=%+v", report)
	}
	if report.Diagnostics[converter.NBTError] != 1 {
		t.Fatalf("diagnostics=%+v", report.Diagnostics)
	}
	if _, err := target.ReadChunk(cellerjava.Overworld, pos); err != nil {
		t.Fatalf("terrain lost: %v", err)
	}
}
