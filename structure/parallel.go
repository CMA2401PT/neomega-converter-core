package structure

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"celler"
	cellerjava "celler/java"
	"celler/java/region"
	"neomega-converter-core/export"
	nbt "neomega-nbt/presets/v1"
)

// encodedChunk 是一根已经编码并压缩好的区块；entities 为空表示这根柱子没有实体。
type encodedChunk struct {
	pos      celler.ChunkPos
	terrain  region.Record
	entities *region.Record
}

type bucketResult struct {
	chunks []encodedChunk
	err    error
}

// writeBuckets 让 worker 独立组装、编码和压缩片区，文件句柄仍由当前协程独占。
// 无缓冲交付使每个 worker 最多持有一个结果；取消后等待所有读者退出再释放映射。
func (s *blockSpool) writeBuckets(world *cellerjava.World, positions []bucketPos, workers int) (int, error) {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	var group sync.WaitGroup
	var next atomic.Int64
	results := make(chan bucketResult)
	for range min(workers, len(positions)) {
		group.Go(func() {
			for {
				if ctx.Err() != nil {
					return
				}
				index := int(next.Add(1) - 1)
				if index >= len(positions) {
					return
				}
				chunks, err := s.encodeBucket(ctx, positions[index])
				select {
				case results <- bucketResult{chunks, err}:
				case <-ctx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		})
	}
	go func() { group.Wait(); close(results) }()
	defer func() { cancel(); group.Wait() }()
	count, completed := 0, 0
	s.progress(Progress{Stage: "writing", Total: int64(len(positions))})
	for result := range results {
		if result.err != nil {
			return count, result.err
		}
		if err := ctx.Err(); err != nil {
			return count, err
		}
		for _, chunk := range result.chunks {
			if err := world.WriteRaw(cellerjava.Overworld, cellerjava.Terrain, chunk.pos, chunk.terrain); err != nil {
				return count, err
			}
			if chunk.entities != nil {
				if err := world.WriteRaw(cellerjava.Overworld, cellerjava.Entities, chunk.pos, *chunk.entities); err != nil {
					return count, err
				}
			}
			count++
		}
		completed++
		s.progress(Progress{Stage: "writing", Completed: int64(completed), Total: int64(len(positions))})
	}
	return count, ctx.Err()
}

// encodeBucket 把一个片区的游程展开成区块柱再编码。可变调色板和编码缓冲只属于当前任务，
// 片区之间只共享源数据。
func (s *blockSpool) encodeBucket(ctx context.Context, p bucketPos) ([]encodedChunk, error) {
	b := s.buckets[p]
	timestamp := uint32(time.Now().Unix())
	built := columns{}
	if err := s.readBucket(ctx, p, b, func(x, y, z, length int32, state uint32) error {
		block := s.palette[state]
		for i := int32(0); i < length; i++ {
			built.place(x+i, y, z, block)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	for _, tile := range b.tiles {
		col := built.at(tile.Pos.Chunk())
		col.blockEntities = append(col.blockEntities, tile)
	}
	for _, entity := range b.entities {
		pos, _ := nbtFloat3(entity["Pos"])
		col := built.at(celler.ChunkPos{X: floorCell(pos[0], 16), Z: floorCell(pos[2], 16)})
		col.entities = append(col.entities, entity)
	}
	height := [2]int32{minWorldY >> 4, (s.highY >> 4) - 1}
	output := make([]encodedChunk, 0, len(built))
	for pos, col := range built {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunk := col.chunk(pos)
		if chunk == nil {
			continue
		}
		terrain, err := export.ToJavaChunk(chunk, export.DataVersion, height)
		if err != nil {
			return nil, err
		}
		data, err := terrain.Encode()
		if err != nil {
			return nil, err
		}
		record, err := region.Compress(data, timestamp)
		if err != nil {
			return nil, err
		}
		encoded := encodedChunk{pos: pos, terrain: record}
		if document := export.ToJavaEntities(chunk, export.DataVersion); document != nil {
			data, err := nbt.EncodeDocument(*document, nbt.EncodingBigEndian)
			if err != nil {
				return nil, err
			}
			record, err := region.Compress(data, timestamp)
			if err != nil {
				return nil, err
			}
			encoded.entities = &record
		}
		output = append(output, encoded)
	}
	return output, nil
}
