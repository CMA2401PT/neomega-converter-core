package javaout

import (
	"time"

	"celler"
	cellerjava "celler/java"
	"celler/java/region"
	"neomega-converter-core"
	nbt "neomega-nbt/presets/v1"
)

// Records 是一根柱子已经编码并压缩好的 region 记录。
// 编码和压缩不访问世界，可以放在并行任务里；Write 才需要独占目标世界。
type Records struct {
	Terrain region.Record
	// Entities 为 nil 表示这根柱子没有实体，不写实体 region。
	Entities *region.Record
	// 实际写出的方块实体数与实体数，超出高度的方块实体不计入。
	BlockEntityCount int
	EntityCount      int
}

// Encode 把柱子编码为地形与实体两条压缩记录。
func Encode(chunk *converter.JavaChunk, height Height) (Records, error) {
	terrain, err := Chunk(chunk, height)
	if err != nil {
		return Records{}, err
	}
	timestamp := uint32(time.Now().Unix())
	data, err := terrain.Encode()
	if err != nil {
		return Records{}, err
	}
	records := Records{}
	if records.Terrain, err = region.Compress(data, timestamp); err != nil {
		return Records{}, err
	}
	blockEntities, _ := nbt.Compounds(terrain.NBT.Value["block_entities"])
	records.BlockEntityCount = len(blockEntities)
	if document := Entities(chunk); document != nil {
		data, err := nbt.EncodeDocument(*document, nbt.EncodingBigEndian)
		if err != nil {
			return Records{}, err
		}
		record, err := region.Compress(data, timestamp)
		if err != nil {
			return Records{}, err
		}
		records.Entities = &record
		records.EntityCount = len(chunk.Entities)
	}
	return records, nil
}

// Write 把两条记录写到目标世界的同一坐标。
func (r Records) Write(world *cellerjava.World, dimension cellerjava.Dimension, pos celler.ChunkPos) error {
	if err := world.WriteRaw(dimension, cellerjava.Terrain, pos, r.Terrain); err != nil {
		return err
	}
	if r.Entities == nil {
		return nil
	}
	return world.WriteRaw(dimension, cellerjava.Entities, pos, *r.Entities)
}
