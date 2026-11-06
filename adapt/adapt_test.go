package adapt

import (
	"context"
	"fmt"
	"io/fs"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter-core"
)

func memoryChunk(t *testing.T, pos celler.ChunkPos, blocks map[layer_storage.Position]ir.IRBlock) *cellerbedrock.Chunk {
	// 测试入口使用真实 Bedrock 记录，覆盖解码与转换的组合。
	t.Helper()
	chunk := cellerbedrock.NewChunk(pos, 2)
	layer := cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:air []", Version: 18168865})
	for local, state := range blocks {
		layer.Blocks.Set(local, cellerbedrock.Block{State: state, Version: 18168865})
	}
	if err := chunk.SetSection(&cellerbedrock.Section{
		Y: 0, Version: 9, Layers: []*cellerbedrock.Layer{layer},
	}); err != nil {
		t.Fatal(err)
	}
	return chunk
}

func memoryRead(chunks map[celler.ChunkPos]*cellerbedrock.Chunk) ReadChunk {
	return func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
		if chunk := chunks[pos]; chunk != nil {
			return chunk, nil
		}
		return nil, fmt.Errorf("chunk %v: %w", pos, fs.ErrNotExist)
	}
}

func getChunk(t *testing.T, src ChunkSource, pos celler.ChunkPos) *converter.JavaChunk {
	t.Helper()
	chunk, err := src.Chunk(context.Background(), pos)
	if err != nil || chunk == nil {
		t.Fatalf("Chunk(%v) = %v, %v", pos, chunk, err)
	}
	return chunk
}

func TestLRUAndOversizedEntries(t *testing.T) {
	raw := memoryChunk(t, celler.ChunkPos{}, nil)
	probe := NewConverted(memoryRead(map[celler.ChunkPos]*cellerbedrock.Chunk{{}: raw}), Options{})
	size := chunkBytes(getChunk(t, probe, celler.ChunkPos{}))
	var reads [3]int
	source := NewConverted(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
		reads[pos.X]++
		copy := *raw
		copy.Pos = pos
		return &copy, nil
	}, Options{CacheBytes: 2 * size})
	for _, x := range []int32{0, 1, 0, 2, 0, 1} {
		getChunk(t, source, celler.ChunkPos{X: x})
	}
	if reads != ([3]int{1, 2, 1}) {
		t.Fatalf("LRU reads=%v", reads)
	}
	for _, limit := range []int64{size - 1, -1} {
		reads := 0
		source := NewConverted(func(context.Context, celler.ChunkPos) (*cellerbedrock.Chunk, error) {
			reads++
			return raw, nil
		}, Options{CacheBytes: limit})
		first := getChunk(t, source, celler.ChunkPos{})
		second := getChunk(t, source, celler.ChunkPos{})
		if reads != 2 || first == second || source.cache.bytes != 0 {
			t.Fatal("超预算或禁用缓存时仍保留了条目")
		}
	}
}

func TestConvertedMetadata(t *testing.T) {
	raw := memoryChunk(t, celler.ChunkPos{X: -1, Z: 2}, nil)
	source := NewConverted(memoryRead(map[celler.ChunkPos]*cellerbedrock.Chunk{raw.Pos: raw}), Options{})
	got := getChunk(t, source, raw.Pos)
	if got.Pos != raw.Pos || got.Dimension != raw.Dimension {
		t.Fatal("转换未保留柱子位置和维度")
	}
	if getChunk(t, source, raw.Pos) != got {
		t.Fatal("转换结果未缓存")
	}
}
