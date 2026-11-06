package structure

import (
	"fmt"
	nbt "neomega-nbt/presets/v1"
)

// streamSchemBlocks 是 Sponge v3 的 Blocks 子标签：调色板、方块数组和方块实体。
type streamSchemBlocks struct {
	Palette       map[string]int32
	Data          *numberStream
	BlockEntities []nbt.Compound
}

// streamSchem 同时接收 Sponge v1/v2（字段在根上）和 v3（字段在 Schematic 下）两种布局。
type streamSchem struct {
	Version       int32
	DataVersion   int32
	Width         uint16
	Height        uint16
	Length        uint16
	Palette       map[string]int32
	BlockData     *numberStream
	BlockEntities []nbt.Compound
	TileEntities  []nbt.Compound
	Entities      []nbt.Compound
	Blocks        streamSchemBlocks
}

// convertSchem 把 Sponge 结构写进 spool。v1/v2 把 BlockData 放在根上，v3 把它放进 Schematic，
// 根上的 Blocks 只在 v3 存在，所以两种布局共用同一个接收器。
func convertSchem(j *job) error {
	h := &streamSchem{BlockData: &numberStream{dir: j.staging, varints: true}}
	h.Blocks.Data = &numberStream{dir: j.staging, varints: true}
	defer h.BlockData.discard()
	defer h.Blocks.Data.discard()
	blocks := &streamCompound{target: &h.Blocks, children: map[string]any{"Data": h.Blocks.Data}}
	v3 := &streamCompound{target: h, children: map[string]any{"BlockData": h.BlockData, "Blocks": blocks}}
	root := &streamCompound{target: h, children: map[string]any{"BlockData": h.BlockData, "Blocks": blocks, "Schematic": v3}}
	if err := readNBT(j.ctx, j.input, root, nbt.EncodingBigEndian, j.report); err != nil {
		return err
	}
	if h.Version < 1 || h.Version > 3 {
		return fmt.Errorf("schem: unsupported version %d", h.Version)
	}
	size := [3]int32{int32(h.Width), int32(h.Height), int32(h.Length)}
	offset, err := heightOffset(0, size[1])
	if err != nil {
		return err
	}
	minimum := [3]int32{0, offset, 0}
	maximum := [3]int32{size[0], size[1] + offset, size[2]}
	palette, data, tiles := h.Palette, h.BlockData, h.BlockEntities
	switch h.Version {
	case 1:
		// v1 的方块实体叫 TileEntities，从 v2 起才叫 BlockEntities。
		tiles = h.TileEntities
	case 3:
		palette, data, tiles = h.Blocks.Palette, h.Blocks.Data, h.Blocks.BlockEntities
	}
	j.spool.palette, err = parseSchemPalette(palette)
	if err != nil {
		return err
	}
	volume := int64(size[0]) * int64(size[1]) * int64(size[2])
	if volume == 0 || data.count != volume {
		return fmt.Errorf("schem: block count %d, expected %d", data.count, volume)
	}
	index := uint64(0)
	if err = data.eachRun(func(value, count uint64) error {
		err := j.spool.linearRun(index, count, size, minimum, uint32(value), false)
		index += count
		return err
	}); err != nil {
		return err
	}
	for _, tile := range tiles {
		pos, ok := tilePosition(tile)
		id := nbtID(tile)
		if !ok || id == "" {
			return fmt.Errorf("schem: invalid block entity Pos/Id")
		}
		j.spool.addTile(javaBlockEntity(pos[0], pos[1]+offset, pos[2], id, spongePayload(tile)))
	}
	for _, raw := range h.Entities {
		if entity, ok := schemEntity(raw); ok {
			entity, _ = shiftEntity(entity, minimum)
			j.spool.addEntity(entity)
		}
	}
	j.spawn = spawnPoint(minimum, maximum)
	return nil
}
