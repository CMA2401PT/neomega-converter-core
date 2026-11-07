package be2je

import (
	"reflect"
	"testing"

	"celler/layer_storage"
	"neomega-blocks/ir"
	converter "neomega-converter-core"
)

func TestStateFallbackPreservesDefaultConversionAndWater(t *testing.T) {
	layers := [][]ir.IRBlock{airLayer(), airLayer()}
	layers[0][0] = "example:preview_block []"
	layers[0][1] = layers[0][0]
	layers[0][2] = "minecraft:stone []"
	layers[1][0] = `minecraft:water ["liquid_depth"=0]`
	source := fixtureChunk(t, layers, nil, 0)
	original, err := ConvertChunk(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(original.Diagnostics) != 1 || original.Diagnostics[0].Kind != converter.UnknownBlock || original.Diagnostics[0].Count != 2 {
		t.Fatalf("default unknown-block diagnostics: %+v", original.Diagnostics)
	}
	assertBlock(t, original.Sections[0].Blocks.Get(layer_storage.At(0, 1, 0)), air)
	nilFallback, err := ConvertChunkWithStateFallback(source, nil, nil)
	if err != nil || !reflect.DeepEqual(original, nilFallback) {
		t.Fatalf("nil fallback changed default conversion: %v", err)
	}
	calls := 0
	preview, err := ConvertChunkWithStateFallback(source, nil, func(state ir.IRBlock) (ir.IRBlock, bool) {
		calls++
		if state != layers[0][0] {
			t.Fatalf("fallback called for recognized block %s", state)
		}
		return "minecraft:cobblestone []", true
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(preview.Diagnostics) != 0 {
		t.Fatalf("calls=%d, diagnostics=%+v", calls, preview.Diagnostics)
	}
	assertBlock(t, preview.Sections[0].Blocks.Get(layer_storage.At(0, 0, 0)), `minecraft:cobblestone ["waterlogged"="true"]`)
	assertBlock(t, preview.Sections[0].Blocks.Get(layer_storage.At(0, 1, 0)), "minecraft:cobblestone []")
	assertBlock(t, preview.Sections[0].Blocks.Get(layer_storage.At(0, 2, 0)), "minecraft:stone []")
	rejected, err := ConvertChunkWithStateFallback(source, nil, func(ir.IRBlock) (ir.IRBlock, bool) {
		return "minecraft:gold_block []", false
	})
	if err != nil || !reflect.DeepEqual(original, rejected) {
		t.Fatalf("rejected fallback changed default conversion: %v", err)
	}
	after, err := ConvertChunk(source)
	if err != nil || !reflect.DeepEqual(original, after) {
		t.Fatalf("preview changed source/default conversion: %v", err)
	}
}
