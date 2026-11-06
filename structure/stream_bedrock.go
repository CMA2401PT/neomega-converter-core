package structure

import (
	"context"
	"encoding/binary"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"celler"
	"celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converter "neomega-converter-core"
	"neomega-converter-core/internal/pipeline"
	beexport "neomega-converter-core/je2be/export"
	rawnbt "neomega-nbt"
	goencode "neomega-nbt/encoder-go-object"
	nbt "neomega-nbt/presets/v1"
)

const (
	// 写入批次的上限：待写记录本身，加上提交时合并 batch 的副本，各占一份。
	batchBytes = 8 * 1024 * 1024
	// 单个片区展开双层方块、编码并压缩所需的工作区估算。
	bucketWorkspaceBytes = 64 * 1024 * 1024
)

// layerStreams 接收 block_indices 的两层数组，每层一个 numberStream。
type layerStreams struct {
	dir     string
	current *numberStream
	layers  []*numberStream
}

func (l *layerStreams) GetReciver(rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	return goencode.ValueReciver{ListConstructor: l}, nil
}
func (l *layerStreams) HasFlag(rawnbt.Flag) bool { return false }

// GetReusableElementPointer 为下一层换一个新的数组接收器。
func (l *layerStreams) GetReusableElementPointer(rawnbt.HighLevelValueType, int, bool) (any, error) {
	l.current = &numberStream{dir: l.dir}
	return l.current, nil
}

// OnNextElement 保存刚写完的一层；current 会被重置给下一层用。
func (l *layerStreams) OnNextElement() error {
	saved := *l.current
	l.layers = append(l.layers, &saved)
	*l.current = numberStream{dir: l.dir}
	return nil
}
func (l *layerStreams) OnListFinish() error { l.layers = append(l.layers, l.current); return nil }
func (l *layerStreams) NonTypedEmptyList() error {
	return fmt.Errorf("mcstructure: missing block_indices")
}
func (l *layerStreams) TypedEmptyList(rawnbt.HighLevelValueType) error { return l.NonTypedEmptyList() }

// discard 释放没有正常收尾的临时文件。
func (l *layerStreams) discard() {
	for _, layer := range l.layers {
		layer.discard()
	}
	if l.current != nil {
		l.current.discard()
	}
}

// bedrockPaletteEntry 是基岩调色板条目：方块名、状态和条目版本。
type bedrockPaletteEntry struct {
	Name    string         `nbt:"name"`
	States  map[string]any `nbt:"states"`
	Version int32          `nbt:"version"`
}

// blockPositionData 是按体积索引排列的方块实体数据。
type blockPositionData struct {
	Entity nbt.Compound `nbt:"block_entity_data"`
}

// nativeStructure 是 .mcstructure 的根标签；双层方块、条目版本和实体都保留原样。
type nativeStructure struct {
	Format    int32    `nbt:"format_version"`
	Origin    [3]int32 `nbt:"structure_world_origin"`
	Size      [3]int32 `nbt:"size"`
	Structure struct {
		Indices  *layerStreams  `nbt:"block_indices"`
		Entities []nbt.Compound `nbt:"entities"`
		Palette  struct {
			Default struct {
				Blocks    []bedrockPaletteEntry        `nbt:"block_palette"`
				Positions map[string]blockPositionData `nbt:"block_position_data"`
			} `nbt:"default"`
		} `nbt:"palette"`
	} `nbt:"structure"`
}

// convertMCStructure 保留基岩双层状态和条目版本，直接写基岩存档，不经过 Java 映射。
func convertMCStructure(j *job) error {
	h := &nativeStructure{}
	h.Structure.Indices = &layerStreams{dir: j.staging}
	defer h.Structure.Indices.discard()
	nested := &streamCompound{target: &h.Structure, children: map[string]any{"block_indices": h.Structure.Indices}}
	root := &streamCompound{target: h, children: map[string]any{"structure": nested}}
	if err := readNBT(j.ctx, j.input, root, nbt.EncodingLittleEndian, j.report); err != nil {
		return err
	}
	if h.Format != 1 {
		return fmt.Errorf("mcstructure: unsupported format_version %d", h.Format)
	}
	for _, n := range h.Size {
		if n <= 0 {
			return fmt.Errorf("mcstructure: invalid size")
		}
	}
	if h.Size[1] > 384 {
		return fmt.Errorf("mcstructure: height %d exceeds Bedrock overworld height 384", h.Size[1])
	}
	volume := int64(h.Size[0]) * int64(h.Size[1]) * int64(h.Size[2])
	offset, err := heightOffset(h.Origin[1], h.Origin[1]+h.Size[1])
	if err != nil {
		return err
	}
	h.Origin[1] += offset
	sink, err := newBlockSpool(j)
	if err != nil {
		return err
	}
	defer sink.discard()
	entries := h.Structure.Palette.Default.Blocks
	if len(entries) == 0 {
		return fmt.Errorf("mcstructure: empty palette")
	}
	// 状态按条目版本写回；两层方块共用一份 spool 调色板。
	blocks := make([]bedrock.Block, len(entries))
	for i, entry := range entries {
		state, err := ir.NewIRBlockFromGo(entry.Name, entry.States)
		if err != nil {
			return err
		}
		blocks[i] = bedrock.Block{State: state.Tidy(), Version: entry.Version}
		sink.palette = append(sink.palette, state.Tidy())
	}
	// 第二层的索引整体偏移一层调色板的长度，写出时按这个步长拆回两层。
	layerStride := len(blocks)
	sink.palette = append(sink.palette, sink.palette...)
	if len(h.Structure.Indices.layers) == 0 || len(h.Structure.Indices.layers) > 2 {
		return fmt.Errorf("mcstructure: invalid layer count")
	}
	for layer, stream := range h.Structure.Indices.layers {
		if stream.count != volume {
			return fmt.Errorf("mcstructure: layer length %d, expected %d", stream.count, volume)
		}
		index := uint64(0)
		err = stream.eachRun(func(value, count uint64) error {
			start := index
			index += count
			id := int64(value)
			if id == -1 {
				return nil // 这一层的这一格没有方块
			}
			if id < 0 || id >= int64(len(blocks)) {
				return fmt.Errorf("mcstructure: palette index %d", id)
			}
			if isAir(blocks[id].State) {
				return nil
			}
			// 基岩体积为 X-Y-Z，连续游程沿 Z 前进。
			for count > 0 {
				z := start % uint64(h.Size[2])
				row := start / uint64(h.Size[2])
				y := row % uint64(h.Size[1])
				x := row / uint64(h.Size[1])
				n := min(count, uint64(h.Size[2])-z)
				state := uint32(id) + uint32(layer*layerStride)
				for dz := uint64(0); dz < n; dz++ {
					if err := sink.run(h.Origin[0]+int32(x), h.Origin[1]+int32(y), h.Origin[2]+int32(z+dz), 1, state, false); err != nil {
						return err
					}
				}
				start += n
				count -= n
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	for key, data := range h.Structure.Palette.Default.Positions {
		if data.Entity == nil {
			continue
		}
		var index int64
		if _, err := fmt.Sscan(key, &index); err != nil || index < 0 || index >= volume {
			return fmt.Errorf("mcstructure: invalid block position %q", key)
		}
		z := int32(index % int64(h.Size[2]))
		row := index / int64(h.Size[2])
		y := int32(row % int64(h.Size[1]))
		x := int32(row / int64(h.Size[1]))
		value := maps.Clone(data.Entity)
		pos := celler.BlockPos{X: x + h.Origin[0], Y: y + h.Origin[1], Z: z + h.Origin[2]}
		value["x"], value["y"], value["z"] = pos.X, pos.Y, pos.Z
		sink.addTile(cellerjava.BlockEntity{Pos: pos, NBT: value})
	}
	for _, raw := range h.Structure.Entities {
		pos, ok := nbtFloat3(raw["Pos"])
		if !ok {
			return fmt.Errorf("mcstructure: entity missing Pos")
		}
		value := maps.Clone(raw)
		value["Pos"] = []float32{float32(pos[0]) + float32(h.Origin[0]), float32(pos[1]) + float32(h.Origin[1]), float32(pos[2]) + float32(h.Origin[2])}
		sink.addEntity(value)
	}
	// 双层区块工作区预留 bucketWorkspaceBytes，写入批次与合并副本各留 batchBytes，其余才用于映射。
	view, err := sink.seal(bucketWorkspaceBytes + 2*batchBytes)
	if err != nil {
		return err
	}
	defer view.Close()
	world, err := bedrock.Open(j.output, bedrock.Options{})
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			world.Close()
		}
	}()
	workers, err := pipeline.Workers(pipeline.Limit{
		MemoryMiB: j.opts.MemoryMiB,
		Requested: j.opts.Workers,
		Reserved:  view.MappedBytes + 2*batchBytes,
		TaskBytes: bucketWorkspaceBytes,
	})
	if err != nil {
		return err
	}
	positions := make([]bucketPos, 0, len(sink.buckets))
	for pos := range sink.buckets {
		positions = append(positions, pos)
	}
	slices.SortFunc(positions, func(a, b bucketPos) int {
		if a.z != b.z {
			return int(a.z - b.z)
		}
		return int(a.x - b.x)
	})
	// 实体 ID 按稳定片区顺序分配，各任务拿到自己的连续编号范围。
	batches := &bedrockBatches{world: world, limit: batchBytes}
	done := int64(0)
	j.report(Progress{Stage: "writing", Total: int64(len(positions))})
	err = pipeline.Run(j.ctx, workers,
		func(ctx context.Context, yield func(bucketTask) error) error {
			actorID := int64(1)
			for _, pos := range positions {
				if err := ctx.Err(); err != nil {
					return err
				}
				current := bucketTask{pos: pos, firstActorID: actorID}
				actorID += int64(len(sink.buckets[pos].entities))
				if err := yield(current); err != nil {
					return err
				}
			}
			return nil
		},
		func(ctx context.Context, current bucketTask) ([]*bedrock.EncodedChunk, error) {
			return sink.encodeBedrockBucket(ctx, current.pos, sink.buckets[current.pos], current.firstActorID, blocks, layerStride)
		},
		func(chunks []*bedrock.EncodedChunk) error {
			if err := batches.add(chunks); err != nil {
				return err
			}
			done++
			j.report(Progress{Stage: "writing", Completed: done, Total: int64(len(positions))})
			return nil
		},
	)
	if err != nil {
		return err
	}
	if err := j.ctx.Err(); err != nil {
		return err
	}
	if err := batches.flush(); err != nil {
		return err
	}
	maximum := [3]int32{h.Origin[0] + h.Size[0], h.Origin[1] + h.Size[1], h.Origin[2] + h.Size[2]}
	j.spawn = spawnPoint(h.Origin, maximum)
	// Bedrock 世界只记录版本和出生点；lastOpenedWithVersion 让游戏按 1.21.50 的格式打开。
	level := bedrock.Level{Version: 10, NBT: nbt.Document{Value: nbt.Compound{
		"StorageVersion": int32(10), "lastOpenedWithVersion": []int32{1, 21, 50, 0}, "LevelName": j.name,
		"SpawnX": j.spawn[0], "SpawnY": j.spawn[1], "SpawnZ": j.spawn[2], "GameType": int32(1),
		"Generator": int32(2), "FlatWorldLayers": `{"biome_id":1,"block_layers":[]}`,
	}}}
	if err = world.WriteLevel(level); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(j.output, "levelname.txt"), []byte(j.name), 0644); err != nil {
		return err
	}
	closed = true
	return world.Close()
}

// bucketTask 是一个待编码的片区；firstActorID 是这个片区第一个实体的编号。
type bucketTask struct {
	pos          bucketPos
	firstActorID int64
}

// bedrockBatches 把编码好的区块攒成批次原子提交。limit 是待写记录的上限；
// 提交时数据库 batch 还会复制一份，所以预算要按两份预留。
type bedrockBatches struct {
	world   *bedrock.World
	pending []*bedrock.EncodedChunk
	size    int
	limit   int
}

// add 把一批区块加入待写队列。超大单柱单独提交；普通批次在加入下一柱前检查大小。
func (b *bedrockBatches) add(chunks []*bedrock.EncodedChunk) error {
	for _, chunk := range chunks {
		if b.size+chunk.Size() > b.limit {
			if err := b.flush(); err != nil {
				return err
			}
		}
		b.pending = append(b.pending, chunk)
		b.size += chunk.Size()
		if b.size >= b.limit {
			if err := b.flush(); err != nil {
				return err
			}
		}
	}
	return nil
}

// flush 提交待写批次。这里的坐标都还没写过，所以跳过旧记录清理。
func (b *bedrockBatches) flush() error {
	if len(b.pending) == 0 {
		return nil
	}
	if err := b.world.WriteChunks(b.pending, bedrock.WriteChunksOptions{NewWorld: true}); err != nil {
		return err
	}
	clear(b.pending)
	b.pending = b.pending[:0]
	b.size = 0
	return nil
}

// encodeBedrockBucket 独立展开一个片区的双层方块，编码完成后交付原生数据库记录。
// actorID 是本片区预分配的首个实体编号，任务之间无需共享可变计数器。
func (sink *blockSpool) encodeBedrockBucket(
	ctx context.Context,
	pos bucketPos,
	b *bucket,
	actorID int64,
	blocks []bedrock.Block,
	layerStride int,
) ([]*bedrock.EncodedChunk, error) {
	chunks := map[celler.ChunkPos]*converter.BedrockChunk{}
	chunkAt := func(p celler.ChunkPos) *converter.BedrockChunk {
		chunk := chunks[p]
		if chunk == nil {
			chunk = &converter.BedrockChunk{Pos: p, Sections: map[int32]*converter.BedrockSection{}}
			chunks[p] = chunk
		}
		return chunk
	}
	err := sink.readBucket(ctx, pos, b, func(x, y, z, length int32, state uint32) error {
		layer, index := int(state)/layerStride, int(state)%layerStride
		for dx := int32(0); dx < length; dx++ {
			chunk := chunkAt(celler.ChunkPos{X: (x + dx) >> 4, Z: z >> 4})
			section := chunk.Sections[y>>4]
			if section == nil {
				empty := bedrock.Block{State: air, Version: blocks[index].Version}
				section = &converter.BedrockSection{Layers: []*layer_storage.Palette[bedrock.Block]{
					layer_storage.MakePaletteStorage(empty),
					layer_storage.MakePaletteStorage(empty),
				}}
				chunk.Sections[y>>4] = section
			}
			section.Layers[layer].Set(layer_storage.At(x+dx, y, z), blocks[index])
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, tile := range b.tiles {
		chunk := chunkAt(tile.Pos.Chunk())
		chunk.BlockEntities = append(chunk.BlockEntities, tile.NBT)
	}
	for _, source := range b.entities {
		entity := maps.Clone(source)
		pos, _ := nbtFloat3(entity["Pos"])
		chunk := chunkAt(celler.ChunkPos{X: floorCell(pos[0], 16), Z: floorCell(pos[2], 16)})
		var key [8]byte
		binary.LittleEndian.PutUint64(key[:], uint64(actorID))
		entity["UniqueID"] = actorID
		actorID++
		chunk.Actors = append(chunk.Actors, converter.BedrockActor{ID: key, NBT: entity})
	}
	output := make([]*bedrock.EncodedChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		encoded, err := beexport.EncodeChunk(chunk)
		if err != nil {
			return nil, err
		}
		output = append(output, encoded)
	}
	return output, nil
}
