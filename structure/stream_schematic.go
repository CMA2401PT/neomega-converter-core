package structure

import (
	"fmt"
	converts "neomega-blocks/presets/block-converts/v1"
	nbt "neomega-nbt/presets/v1"
)

// streamLegacy 是 Alpha schematic 的根标签。Blocks、Data、AddBlocks 是三个并行的方块数组。
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

// convertSchematic 把旧数字 ID 的 schematic 写成 Java 存档。
// Blocks 给低 8 位 ID，AddBlocks 每方块补 4 位高位，Data 给 meta；
// 三个数组各自成游程，合并时每次只推进最短的那一段。
func convertSchematic(j *job) error {
	h := &streamLegacy{
		Blocks:    &numberStream{dir: j.staging},
		Data:      &numberStream{dir: j.staging},
		AddBlocks: &numberStream{dir: j.staging},
	}
	defer h.Blocks.discard()
	defer h.Data.discard()
	defer h.AddBlocks.discard()
	root := &streamCompound{target: h, children: map[string]any{"Blocks": h.Blocks, "Data": h.Data, "AddBlocks": h.AddBlocks}}
	if err := readNBT(j.ctx, j.input, root, nbt.EncodingBigEndian, j.report); err != nil {
		return err
	}
	size := [3]int32{int32(h.Width), int32(h.Height), int32(h.Length)}
	volume := int64(size[0]) * int64(size[1]) * int64(size[2])
	if volume == 0 || h.Blocks.count != volume || h.Data.count != volume {
		return fmt.Errorf("schematic: invalid size or Blocks/Data length")
	}
	if h.Materials != "" && h.Materials != "Alpha" {
		return fmt.Errorf("schematic: unsupported Materials %q", h.Materials)
	}
	offset, err := heightOffset(0, size[1])
	if err != nil {
		return err
	}
	minimum := [3]int32{0, offset, 0}
	maximum := [3]int32{size[0], size[1] + offset, size[2]}
	ids, err := h.Blocks.open()
	if err != nil {
		return err
	}
	defer ids.close()
	meta, err := h.Data.open()
	if err != nil {
		return err
	}
	defer meta.close()
	var high *runReader
	if h.AddBlocks.temp != nil {
		if h.AddBlocks.count != (volume+1)/2 {
			return fmt.Errorf("schematic: AddBlocks length")
		}
		if high, err = h.AddBlocks.open(); err != nil {
			return err
		}
		defer high.close()
	}
	// id<<4|meta 只对应有限几种方块，先查映射，避免同一个状态重复进调色板。
	mapping := make(map[uint16]uint32)
	var highByte uint64
	for index := uint64(0); index < uint64(volume); {
		if ids.remaining == 0 {
			if err = ids.next(); err != nil {
				return err
			}
		}
		if meta.remaining == 0 {
			if err = meta.next(); err != nil {
				return err
			}
		}
		n := min(ids.remaining, meta.remaining)
		id := uint16(ids.value)
		if high != nil {
			// AddBlocks 一个字节放两个方块的高 4 位：偶数格在低半字节，奇数格在高半字节。
			n = 1
			if index%2 == 0 {
				if highByte, err = high.one(); err != nil {
					return err
				}
			}
			id |= uint16(highByte>>(4*(index%2))&15) << 8
		}
		metaValue := uint8(meta.value & 15)
		key := id<<4 | uint16(metaValue)
		paletteIndex, found := mapping[key]
		if !found {
			block, ok := converts.SchematicToJava(id, uint16(metaValue))
			if !ok {
				return fmt.Errorf("schematic: unknown block %d:%d", id, metaValue)
			}
			paletteIndex = uint32(len(j.spool.palette))
			mapping[key] = paletteIndex
			j.spool.palette = append(j.spool.palette, block)
		}
		if err = j.spool.linearRun(index, n, size, minimum, paletteIndex, false); err != nil {
			return err
		}
		ids.remaining -= n
		meta.remaining -= n
		index += n
	}
	for _, tile := range h.TileEntities {
		pos, ok := tilePosition(tile)
		id := nbtID(tile)
		if !ok || id == "" {
			return fmt.Errorf("schematic: invalid tile entity")
		}
		j.spool.addTile(javaBlockEntity(pos[0], pos[1]+offset, pos[2], id, spongePayload(tile)))
	}
	for _, raw := range h.Entities {
		if entity, ok := shiftEntity(raw, minimum); ok {
			j.spool.addEntity(entity)
		}
	}
	j.spawn = spawnPoint(minimum, maximum)
	return nil
}
