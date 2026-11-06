package structure

import (
	"fmt"
	converts "neomega-blocks/presets/block-converts/v1"
	nbt "neomega-nbt/presets/v1"
)

type streamLegacy struct {
	Width        uint16
	Height       uint16
	Length       uint16
	Materials    string
	Blocks       *numberStream
	Data         *numberStream
	AddBlocks    *numberStream
	TileEntities []nbt.Compound
	Entities     []nbt.Compound
}

func streamSchematicFile(path, temp string, sink *blockSpool) ([3]int32, error) {
	h := &streamLegacy{Blocks: &numberStream{dir: temp}, Data: &numberStream{dir: temp}, AddBlocks: &numberStream{dir: temp}}
	defer func() {
		for _, s := range []*numberStream{h.Blocks, h.Data, h.AddBlocks} {
			if s.file != nil {
				s.file.Close()
			}
		}
	}()
	if err := readNBT(sink.ctx, path, &streamCompound{target: h, children: map[string]any{"Blocks": h.Blocks, "Data": h.Data, "AddBlocks": h.AddBlocks}}, nbt.EncodingBigEndian, sink.progress); err != nil {
		return [3]int32{}, err
	}
	size := [3]int32{int32(h.Width), int32(h.Height), int32(h.Length)}
	volume := int64(size[0]) * int64(size[1]) * int64(size[2])
	if volume == 0 || h.Blocks.count != volume || h.Data.count != volume {
		return [3]int32{}, fmt.Errorf("schematic: invalid size or Blocks/Data length")
	}
	if h.Materials != "" && h.Materials != "Alpha" {
		return [3]int32{}, fmt.Errorf("schematic: unsupported Materials %q", h.Materials)
	}
	offset, err := heightOffset(0, size[1])
	if err != nil {
		return [3]int32{}, err
	}
	minimum := [3]int32{0, offset, 0}
	maximum := [3]int32{size[0], size[1] + offset, size[2]}
	ids, err := h.Blocks.open()
	if err != nil {
		return [3]int32{}, err
	}
	defer ids.close()
	meta, err := h.Data.open()
	if err != nil {
		return [3]int32{}, err
	}
	defer meta.close()
	var high *runReader
	if h.AddBlocks.path != "" {
		if h.AddBlocks.count != (volume+1)/2 {
			return [3]int32{}, fmt.Errorf("schematic: AddBlocks length")
		}
		high, err = h.AddBlocks.open()
		if err != nil {
			return [3]int32{}, err
		}
		defer high.close()
	}
	// 旧格式的 Blocks 与 Data 分开保存。合并两个游程流，不展开成等体积数组。
	mapping := make(map[uint16]uint32)
	var highByte uint64
	for index := uint64(0); index < uint64(volume); {
		if ids.remaining == 0 {
			if err = ids.next(); err != nil {
				return [3]int32{}, err
			}
		}
		if meta.remaining == 0 {
			if err = meta.next(); err != nil {
				return [3]int32{}, err
			}
		}
		n := min(ids.remaining, meta.remaining)
		id := uint16(ids.value)
		if high != nil {
			n = 1
			if index%2 == 0 {
				highByte, err = high.one()
				if err != nil {
					return [3]int32{}, err
				}
			}
			id |= uint16(highByte>>(4*(index%2))&15) << 8
		}
		key := id<<4 | uint16(meta.value&15)
		state, found := mapping[key]
		if !found {
			block, ok := converts.SchematicToJava(id, key&15)
			if !ok {
				return [3]int32{}, fmt.Errorf("schematic: unknown block %d:%d", id, key&15)
			}

			state = uint32(len(sink.palette))
			mapping[key] = state
			sink.palette = append(sink.palette, block)
		}
		if err = sink.linearRun(index, n, size, minimum, state, false); err != nil {
			return [3]int32{}, err
		}
		ids.remaining -= n
		meta.remaining -= n
		index += n
	}
	for _, tile := range h.TileEntities {
		x, okX := nbtInt32(tile["x"])
		y, okY := nbtInt32(tile["y"])
		z, okZ := nbtInt32(tile["z"])
		id, _ := tile["id"].(string)
		// 部分导出器在 Alpha 方块数组旁写 Sponge 的 Pos/Id 包装。
		payload := tile
		if pos, ok := int3(tile["Pos"]); ok && len(pos) == 3 {
			x, y, z = pos[0], pos[1], pos[2]
			okX, okY, okZ = true, true, true
			id, _ = tile["Id"].(string)
			if data, ok := tile["Data"].(map[string]any); ok {
				payload = data
			}
		}
		if !okX || !okY || !okZ || id == "" {
			return [3]int32{}, fmt.Errorf("schematic: invalid tile entity Pos/Id or x/y/z/id")
		}
		sink.addTile(javaBlockEntity(x, y+offset, z, id, payload))
	}
	for _, raw := range h.Entities {
		if entity, ok := shiftEntity(raw, minimum); ok {
			sink.addEntity(entity)
		}
	}
	return spawnAbove(minimum, maximum), nil
}
