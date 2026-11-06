package structure

import (
	"fmt"
	nbt "neomega-nbt/presets/v1"
)

type streamSchemBlocks struct {
	Palette       map[string]int32
	Data          *numberStream
	BlockEntities []nbt.Compound
}
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

func streamSchemFile(path, temp string, sink *blockSpool) ([3]int32, error) {
	h := &streamSchem{BlockData: &numberStream{dir: temp, varints: true}, Blocks: streamSchemBlocks{Data: &numberStream{dir: temp, varints: true}}}
	body := &streamCompound{target: h, children: map[string]any{"BlockData": h.BlockData, "Blocks": &streamCompound{target: &h.Blocks, children: map[string]any{"Data": h.Blocks.Data}}}}
	root := &streamCompound{target: h, children: map[string]any{"BlockData": h.BlockData, "Blocks": body.children["Blocks"], "Schematic": body}}
	defer func() {
		if h.BlockData.file != nil {
			h.BlockData.file.Close()
		}
		if h.Blocks.Data.file != nil {
			h.Blocks.Data.file.Close()
		}
	}()
	if err := readNBT(sink.ctx, path, root, nbt.EncodingBigEndian, sink.progress); err != nil {
		return [3]int32{}, err
	}
	if h.Version < 1 || h.Version > 3 {
		return [3]int32{}, fmt.Errorf("schem: unsupported version %d", h.Version)
	}
	size := [3]int32{int32(h.Width), int32(h.Height), int32(h.Length)}
	offset, err := heightOffset(0, size[1])
	if err != nil {
		return [3]int32{}, err
	}
	minimum := [3]int32{0, offset, 0}
	maximum := [3]int32{size[0], size[1] + offset, size[2]}
	palette, data, tiles := h.Palette, h.BlockData, h.BlockEntities
	if h.Version == 1 {
		tiles = h.TileEntities
	}
	if h.Version == 3 {
		palette, data, tiles = h.Blocks.Palette, h.Blocks.Data, h.Blocks.BlockEntities
	}
	raw := make(map[string]any, len(palette))
	for name, id := range palette {
		raw[name] = id
	}
	sink.palette, err = parseSchemPalette(raw)
	if err != nil {
		return [3]int32{}, err
	}
	volume := int64(size[0]) * int64(size[1]) * int64(size[2])
	if volume == 0 || data.count != volume {
		return [3]int32{}, fmt.Errorf("schem: block count %d, expected %d", data.count, volume)
	}
	index := uint64(0)
	err = data.eachRun(func(value, count uint64) error {
		err := sink.linearRun(index, count, size, minimum, uint32(value), false)
		index += count
		return err
	})
	if err != nil {
		return [3]int32{}, err
	}
	for _, tile := range tiles {
		pos, ok := int3(tile["Pos"])
		id, _ := tile["Id"].(string)
		if id == "" {
			id, _ = tile["id"].(string)
		}
		if !ok || len(pos) != 3 || id == "" {
			return [3]int32{}, fmt.Errorf("schem: invalid block entity Pos/Id")
		}
		payload := tile
		if data, ok := tile["Data"].(map[string]any); ok {
			payload = data
		}
		sink.addTile(javaBlockEntity(pos[0], pos[1]+offset, pos[2], id, payload))
	}
	for _, raw := range h.Entities {
		if entity, ok := schemEntity(raw); ok {
			entity, _ = shiftEntity(entity, minimum)
			sink.addEntity(entity)
		}
	}
	return spawnAbove(minimum, maximum), nil
}
