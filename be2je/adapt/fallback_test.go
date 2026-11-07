package adapt

import (
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
)

func TestStateFallbackIsForwardedAndCached(t *testing.T) {
	pos := celler.ChunkPos{}
	at := layer_storage.At(1, 2, 3)
	raw := memoryChunk(t, pos, map[layer_storage.Position]ir.IRBlock{at: "example:preview_block []"})
	read := memoryRead(map[celler.ChunkPos]*cellerbedrock.Chunk{pos: raw})
	calls := 0
	source := NewConverted(read, Options{StateFallback: func(ir.IRBlock) (ir.IRBlock, bool) {
		calls++
		return "minecraft:stone []", true
	}})
	first := getChunk(t, source, pos)
	second := getChunk(t, source, pos)
	if calls != 1 || first != second || first.Sections[0].Blocks.Get(at) != "minecraft:stone []" || len(first.Diagnostics) != 0 {
		t.Fatalf("fallback/cache: calls=%d first=%+v second=%+v", calls, first, second)
	}
	defaultResult := getChunk(t, NewConverted(read, Options{}), pos)
	if defaultResult.Sections[0].Blocks.Get(at) != "minecraft:air []" || len(defaultResult.Diagnostics) != 1 {
		t.Fatalf("default adapter changed: %+v", defaultResult)
	}
}

func TestFrameItemResolverIsForwarded(t *testing.T) {
	pos := celler.ChunkPos{}
	raw := memoryChunk(t, pos, map[layer_storage.Position]ir.IRBlock{layer_storage.At(1, 2, 3): `minecraft:frame ["facing_direction"=3]`})
	if err := raw.SetBlockEntities([]nbt.Document{{Value: nbt.Compound{"id": "ItemFrame", "x": int32(1), "y": int32(2), "z": int32(3), "Item": nbt.Compound{"Name": "example:new_item", "Count": int8(1)}}}}); err != nil {
		t.Fatal(err)
	}
	read := memoryRead(map[celler.ChunkPos]*cellerbedrock.Chunk{pos: raw})
	source := NewConverted(read, Options{FrameItemResolver: func(nbt.Compound, string, bool) (string, bool) { return "minecraft:stone", true }})
	got := getChunk(t, source, pos)
	if len(got.Entities) != 1 || len(got.Diagnostics) != 0 || got.Entities[0]["Item"].(nbt.Compound)["id"] != "minecraft:stone" {
		t.Fatalf("resolver not forwarded: %+v", got)
	}
	legacy := getChunk(t, NewConverted(read, Options{}), pos)
	if len(legacy.Entities) != 1 || legacy.Entities[0]["Item"] != nil || len(legacy.Diagnostics) != 1 {
		t.Fatalf("legacy behavior changed: %+v", legacy)
	}
}
