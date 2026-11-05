package export

import (
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter-core"
	converterjava "neomega-converter-core/je2be"
	nbt "neomega-nbt/presets/v1"
)

func javaChestChunk(t *testing.T) *cellerjava.Chunk {
	t.Helper()
	chunk := cellerjava.NewChunk(celler.ChunkPos{}, 4189)
	blocks := layer_storage.MakePaletteStorage(ir.IRBlock(`minecraft:chest ["facing"="north","type"="single","waterlogged"="false"]`))
	if err := chunk.SetSection(&cellerjava.Section{Y: 0, Blocks: blocks}); err != nil {
		t.Fatal(err)
	}
	chunk.NBT.Value["block_entities"] = []nbt.Compound{{
		"id": "minecraft:chest", "x": int32(0), "y": int32(0), "z": int32(0),
	}}
	return chunk
}

func openBedrock(t *testing.T) *cellerbedrock.World {
	t.Helper()
	world, err := cellerbedrock.Open(t.TempDir(), cellerbedrock.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = world.Close() })
	return world
}

func TestWriteChunk(t *testing.T) {
	converted, err := converterjava.ConvertChunk(celler.ChunkPos{}, 0, javaChestChunk(t))
	if err != nil {
		t.Fatal(err)
	}
	world := openBedrock(t)
	if err := WriteChunk(world, converted); err != nil {
		t.Fatal(err)
	}
	back, err := world.ReadChunk(0, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	section, err := back.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := section.Layers[0].Blocks.Get(layer_storage.At(3, 3, 3)).State.NameWithNameSpace(); got != "minecraft:chest" {
		t.Fatalf("block: %s", got)
	}
	entities, err := back.BlockEntities()
	if err != nil || len(entities) != 1 || entities[0].Value["id"] != "Chest" {
		t.Fatalf("entities: %#v, %v", entities, err)
	}
	biomes, err := back.Biomes3D()
	if err != nil || len(biomes.Sections) != 24 {
		t.Fatalf("biomes: %#v, %v", biomes, err)
	}
	if finalized := back.Records[string([]byte{cellerbedrock.TagFinalizedState})]; len(finalized) != 1 || finalized[0] != 2 {
		t.Fatalf("finalized: %x", finalized)
	}
	if version := back.Records[string([]byte{cellerbedrock.TagVersion})]; len(version) != 1 || version[0] != chunkVersion {
		t.Fatalf("version: %x", version)
	}
}

func TestConvertLevel(t *testing.T) {
	document := nbt.Document{Value: nbt.Compound{"Data": nbt.Compound{
		"LevelName": "world", "SpawnX": int32(1), "SpawnY": int32(64), "SpawnZ": int32(-2),
		"GameType": int32(1), "Difficulty": int8(3), "hardcore": int8(1), "allowCommands": int8(1),
		"Time": int64(100), "DayTime": int64(200), "LastPlayed": int64(1_700_000_000_000),
		"raining": int8(1), "thundering": int8(0), "rainTime": int32(10), "thunderTime": int32(20),
		"GameRules": nbt.Compound{"keepInventory": "true", "randomTickSpeed": int32(5)},
	}}}
	level, report := ConvertLevel(document)
	if level.Version != 10 {
		t.Fatalf("version: %d", level.Version)
	}
	value := level.NBT.Value
	for key, want := range map[string]any{
		"LevelName": "world", "SpawnX": int32(1), "SpawnY": int32(64), "SpawnZ": int32(-2),
		"GameType": int32(1), "Difficulty": int32(3), "IsHardcore": int8(1), "commandsEnabled": int8(1),
		"currentTick": int64(100), "Time": int64(200), "LastPlayed": int64(1_700_000_000),
		"rainLevel": float32(1), "lightningLevel": float32(0), "rainTime": int32(10), "lightningTime": int32(20),
		"StorageVersion": int32(10), "Generator": int32(1), "GenerateStructures": int8(1),
		"keepinventory": int8(1), "randomtickspeed": int32(5),
	} {
		if value[key] != want {
			t.Errorf("%s = %#v, want %#v", key, value[key], want)
		}
	}
	if len(report.Defaulted) == 0 {
		t.Fatal("expected defaulted fields")
	}
}

func TestConvertWorld(t *testing.T) {
	dir := t.TempDir()
	source, err := cellerjava.OpenDirectory(dir, cellerjava.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.WriteChunk(cellerjava.Overworld, javaChestChunk(t)); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := cellerjava.OpenDirectory(dir, cellerjava.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	target := openBedrock(t)
	report, err := ConvertWorld(reader, target, ConvertOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Chunks[0] != 1 || len(report.FailedChunks) != 0 {
		t.Fatalf("report: %+v", report)
	}
	if _, err := target.ReadChunk(0, celler.ChunkPos{}); err != nil {
		t.Fatalf("chunk not written: %v", err)
	}
}

func TestWriteChunkActors(t *testing.T) {
	converted, err := converterjava.ConvertChunk(celler.ChunkPos{}, 0, javaChestChunk(t))
	if err != nil {
		t.Fatal(err)
	}
	converted.Actors = []converter.BedrockActor{{
		ID:  [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		NBT: nbt.Compound{"identifier": "minecraft:zombie"},
	}}
	world := openBedrock(t)
	if err := WriteChunk(world, converted); err != nil {
		t.Fatal(err)
	}
	documents, err := world.ReadChunkActors(0, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0].Value["identifier"] != "minecraft:zombie" {
		t.Fatalf("actors: %#v", documents)
	}
}
