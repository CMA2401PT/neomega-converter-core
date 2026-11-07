package adapt

import (
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
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
