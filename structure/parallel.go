package structure

import (
	"context"
	"time"

	"celler"
	cellerjava "celler/java"
	"celler/java/region"
	"neomega-converter-core/export"
	"neomega-converter-core/internal/pipeline"
	nbt "neomega-nbt/presets/v1"
)

// encodedChunk 是一根已经编码并压缩好的区块；entities 为空表示这根柱子没有实体。
type encodedChunk struct {
	pos      celler.ChunkPos
	terrain  region.Record
	entities *region.Record
}

// writeBuckets 并行组装、编码和压缩片区，按空间顺序把结果交给当前协程写入。
// 共用有界调度器，取消后等待所有读者退出再释放映射。
func (s *blockSpool) writeBuckets(world *cellerjava.World, positions []bucketPos, workers int) (int, error) {
	count, completed := 0, 0
	s.progress(Progress{Stage: "writing", Total: int64(len(positions))})
	err := pipeline.Run(s.ctx, workers,
		func(ctx context.Context, yield func(bucketPos) error) error {
			for _, pos := range positions {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := yield(pos); err != nil {
					return err
				}
			}
			return nil
		},
		s.encodeBucket,
		func(chunks []encodedChunk) error {
			for _, chunk := range chunks {
				if err := world.WriteRaw(cellerjava.Overworld, cellerjava.Terrain, chunk.pos, chunk.terrain); err != nil {
					return err
				}
				if chunk.entities != nil {
					if err := world.WriteRaw(cellerjava.Overworld, cellerjava.Entities, chunk.pos, *chunk.entities); err != nil {
						return err
					}
				}
				count++
			}
			completed++
			s.progress(Progress{Stage: "writing", Completed: int64(completed), Total: int64(len(positions))})
			return nil
		},
	)
	return count, err
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
