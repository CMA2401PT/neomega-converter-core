// Package structure 把有限体积的结构文件写成 Java 或 Bedrock 存档。
// Java 格式保留 Java 状态，mcstructure 保留基岩状态和双层方块。
package structure

import (
	"fmt"

	"celler"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter-core"
	nbt "neomega-nbt/presets/v1"
)

const air ir.IRBlock = "minecraft:air []"

// 默认主世界高度。更高的 Java 结构由数据包声明扩展范围。
const (
	minWorldY int32 = -64
	maxWorldY int32 = 320
)

// 没有群系的结构统一写 plains，Java section 需要这个字段。
var plainsBiomes = func() *[64]string {
	var biomes [64]string
	for i := range biomes {
		biomes[i] = "minecraft:plains"
	}
	return &biomes
}()

type column struct {
	sections      map[int32]*layer_storage.Palette[ir.IRBlock]
	blockEntities []cellerjava.BlockEntity
	entities      []nbt.Compound
}

func place(columns map[celler.ChunkPos]*column, x, y, z int32, block ir.IRBlock) {
	pos := celler.ChunkPos{X: x >> 4, Z: z >> 4}
	sectionY := y >> 4
	if isAir(block) {
		// 后来的区域可以用空气盖住先前的方块，纯空气则不必新建 section。
		col := columns[pos]
		if col == nil || col.sections[sectionY] == nil {
			return
		}
		col.sections[sectionY].Set(layer_storage.At(x, y, z), air)
		return
	}
	col := ensure(columns, pos)
	palette := col.sections[sectionY]
	if palette == nil {
		palette = layer_storage.MakePaletteStorage(air)
		col.sections[sectionY] = palette
	}
	palette.Set(layer_storage.At(x, y, z), block)
}

func ensure(columns map[celler.ChunkPos]*column, pos celler.ChunkPos) *column {
	col := columns[pos]
	if col == nil {
		col = &column{sections: map[int32]*layer_storage.Palette[ir.IRBlock]{}}
		columns[pos] = col
	}
	return col
}

func (col *column) converted(pos celler.ChunkPos) *converter.ConvertedChunk {
	chunk := &converter.ConvertedChunk{
		Pos:           pos,
		Sections:      map[int32]*converter.ConvertedSection{},
		BlockEntities: col.blockEntities,
		Entities:      col.entities,
	}
	for y, blocks := range col.sections {
		if !hasNonAir(blocks) {
			continue
		}
		chunk.Sections[y] = &converter.ConvertedSection{Blocks: blocks, Biomes: plainsBiomes}
	}
	if len(chunk.Sections) == 0 && len(chunk.BlockEntities) == 0 && len(chunk.Entities) == 0 {
		return nil
	}
	return chunk
}

func hasNonAir(blocks *layer_storage.Palette[ir.IRBlock]) bool {
	// 调色板会留下被盖掉的旧方块。只看格子里还在的非空气。
	found := false
	blocks.IterNonDefault(func(_ layer_storage.Position, block ir.IRBlock) bool {
		if !isAir(block) {
			found = true
			return false
		}
		return true
	})
	return found
}

func isAir(block ir.IRBlock) bool {
	switch block {
	case "", "minecraft:air []", "minecraft:cave_air []", "minecraft:void_air []":
		return true
	default:
		return false
	}
}

// spawnAbove 把出生点放在结构顶面上方两格，打开时不会站在方块里面。
func spawnAbove(min, max [3]int32) [3]int32 {
	return [3]int32{
		min[0] + (max[0]-min[0])/2,
		max[1] + 2,
		min[2] + (max[2]-min[2])/2,
	}
}

func floorDiv(value, step float64) int {
	q := int(value / step)
	if value < 0 && float64(q)*step != value {
		q--
	}
	return q
}

// heightOffset 尽量保留源高度；需要时整体平移，使 384 格高的结构也能完整放入主世界。
func heightOffset(minY, maxY int32) (int32, error) {
	if maxY-minY > 2096 {
		return 0, fmt.Errorf("structure: height %d exceeds supported Java height 2096", maxY-minY)
	}
	if maxY-minY > 384 {
		return minWorldY - minY, nil
	}
	if minY < minWorldY {
		return minWorldY - minY, nil
	}
	if maxY > maxWorldY {
		return maxWorldY - maxY, nil
	}
	return 0, nil
}
