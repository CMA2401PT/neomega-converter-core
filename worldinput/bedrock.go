package worldinput

import (
	"celler"
	"celler/bedrock"
	"context"
	"errors"
	"io/fs"
	"neomega-converter-core/be2je/adapt"
	nbt "neomega-nbt/presets/v1"
	"sync"
)

// BedrockReads 让转换缓存的后台读取在数据库关闭前结束。
type BedrockReads struct {
	// adapt 的等待者取消后，回调仍可能在读取；计数只围住原始读取，不限制其并发。
	mu              sync.Mutex
	active          sync.WaitGroup
	closed          bool
	ReadChunk       adapt.ReadChunk
	ReadChunkActors adapt.ReadActors
	ScanChunks      func(context.Context) ([]celler.ChunkPos, error)
}

// Read 登记原始 I/O，而非只登记可能提前取消的转换等待者。
func (r *BedrockReads) Read(ctx context.Context, pos celler.ChunkPos) (*bedrock.Chunk, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, fs.ErrClosed
	}
	r.active.Add(1)
	r.mu.Unlock()
	defer r.active.Done()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.ReadChunk(ctx, pos)
}

// ReadActors 与地形读取共用关闭屏障，取消等待后也不会读取已关闭的数据库。
func (r *BedrockReads) ReadActors(ctx context.Context, pos celler.ChunkPos) ([]nbt.Compound, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, fs.ErrClosed
	}
	r.active.Add(1)
	r.mu.Unlock()
	defer r.active.Done()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.ReadChunkActors(ctx, pos)
}

// Chunks 将索引迭代器纳入关闭等待。
func (r *BedrockReads) Chunks(ctx context.Context) ([]celler.ChunkPos, error) {
	// 索引只读取键，也需要在关闭数据库前归还迭代器。
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, fs.ErrClosed
	}
	r.active.Add(1)
	r.mu.Unlock()
	defer r.active.Done()
	return r.ScanChunks(ctx)
}

// Close 拒绝迟到的转换回调，并等待已有读取归还数据库。
func (r *BedrockReads) Close() {
	// 返回时原始读取已结束；迟到的后台回调也不能重新进入已归还的句柄。
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	r.active.Wait()
}

// bedrockPosition 在出生点没有区块时，寻找所选维度中的已有内容。
func bedrockPosition(world *bedrock.World, dimension int32) ([3]float64, error) {
	// 出生点可缺省；回退扫描在所选维度找到第一根柱子就停止，避免启动时读整张图。
	position := [3]float64{0, 80, 0}
	offset := float64(0)
	if level, err := world.ReadLevel(); err == nil {
		offset = 12
		for axis, name := range []string{"SpawnX", "SpawnY", "SpawnZ"} {
			if value, ok := level.NBT.Value[name].(int32); ok {
				position[axis] = float64(value)
			}
		}
	}
	pos := celler.ChunkPos{
		X: int32(position[0]) >> 4,
		Z: int32(position[2]) >> 4,
	}
	chunk, err := world.ReadChunk(dimension, pos)
	if !errors.Is(err, fs.ErrNotExist) {
		if err != nil {
			return position, err
		}
		minY, height := [3]int{-64, 0, 0}[dimension], [3]int{384, 128, 256}[dimension]
		if position[1] < float64(minY) || position[1] >= float64(minY+height) {
			position[1] = bedrockSurface(chunk)
		} else {
			position[1] += offset
		}
		return position, nil
	}
	found := errors.New("found first chunk")
	err = world.ScanChunks(func(chunk *bedrock.Chunk) error {
		if chunk.Dimension != dimension {
			return nil
		}
		position[0] = float64(chunk.Pos.X)*16 + 8
		position[2] = float64(chunk.Pos.Z)*16 + 8
		position[1] = bedrockSurface(chunk)
		return found
	})
	if errors.Is(err, found) {
		err = nil
	}
	return position, err
}

// bedrockSurface 用最高非空气 section 估算落点，让游戏的自动找地面出生点也能预览。
func bedrockSurface(chunk *bedrock.Chunk) float64 {
	highest := int32(-129)
	for _, y := range chunk.SectionYs() {
		section, err := chunk.Section(y)
		if err != nil {
			continue
		}
		for _, layer := range section.Layers {
			for _, block := range layer.Blocks.GetUsedPalette() {
				switch block.State.NameWithNameSpace() {
				case "minecraft:air", "minecraft:cave_air", "minecraft:void_air":
				default:
					highest = max(highest, y)
				}
			}
		}
	}
	if highest >= -128 {
		return float64(highest*16 + 24)
	}
	return 80
}

// BedrockChunks 只扫描键，避免为了占用表解码整个世界。
func BedrockChunks(ctx context.Context, world *bedrock.World, dimension int32) ([]celler.ChunkPos, error) {
	// 同一柱子通常有多个键，只扫描键并按维度去重，避免为远景解码整个世界。
	iter := world.DB.NewIterator(nil, nil)
	defer iter.Release()
	seen := map[celler.ChunkPos]bool{}
	positions := make([]celler.ChunkPos, 0)
	for iter.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pos, dim, _, ok := bedrock.ParseChunkKey(iter.Key())
		if ok && dim == dimension && !seen[pos] {
			seen[pos] = true
			positions = append(positions, pos)
		}
	}
	return positions, iter.Error()
}
