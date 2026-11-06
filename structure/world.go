// Package structure 把有限体积的结构文件写成 Java 或 Bedrock 存档。
// Java 格式保留 Java 状态，mcstructure 保留基岩状态和双层方块。
package structure

import (
	"fmt"
	"math"

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

// column 是一根区块柱：按 section 存的方块、方块实体和实体。
type column struct {
	sections      map[int32]*layer_storage.Palette[ir.IRBlock]
	blockEntities []cellerjava.BlockEntity
	entities      []nbt.Compound
}

// columns 是按区块坐标索引的一批柱子。方块按源文件的扫描顺序进来，先合并到这里再写区块。
type columns map[celler.ChunkPos]*column

// at 取一根柱子，没有就建一根空的。
func (c columns) at(pos celler.ChunkPos) *column {
	col := c[pos]
	if col == nil {
		col = &column{sections: map[int32]*layer_storage.Palette[ir.IRBlock]{}}
		c[pos] = col
	}
	return col
}

// place 写入一格方块。空气只覆盖已存在的位置，让后来的区域能盖住先前的方块。
func (c columns) place(x, y, z int32, block ir.IRBlock) {
	pos := celler.ChunkPos{X: x >> 4, Z: z >> 4}
	sectionY := y >> 4
	if isAir(block) {
		if col := c[pos]; col != nil && col.sections[sectionY] != nil {
			col.sections[sectionY].Set(layer_storage.At(x, y, z), air)
		}
		return
	}
	col := c.at(pos)
	palette := col.sections[sectionY]
	if palette == nil {
		palette = layer_storage.MakePaletteStorage(air)
		col.sections[sectionY] = palette
	}
	palette.Set(layer_storage.At(x, y, z), block)
}

// chunk 组装一根可写出的区块；没有方块、方块实体和实体时返回 nil。
func (col *column) chunk(pos celler.ChunkPos) *converter.ConvertedChunk {
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

// hasNonAir 报告一个 section 里是否还留着非空气方块。
// 调色板会留下被盖掉的旧方块，所以只能看格子里现在的值。
func hasNonAir(blocks *layer_storage.Palette[ir.IRBlock]) bool {
	found := false
	blocks.IterNonDefault(func(_ layer_storage.Position, block ir.IRBlock) bool {
		found = !isAir(block)
		return !found
	})
	return found
}

// isAir 判断方块是否为空气；结构文件里的空气有三种写法，空串表示调色板空洞。
func isAir(block ir.IRBlock) bool {
	switch block {
	case "", "minecraft:air []", "minecraft:cave_air []", "minecraft:void_air []":
		return true
	default:
		return false
	}
}

// spawnPoint 把出生点放在结构顶面中心上方两格，打开时不会站在方块里面。
func spawnPoint(min, max [3]int32) [3]int32 {
	return [3]int32{
		min[0] + (max[0]-min[0])/2,
		max[1] + 2,
		min[2] + (max[2]-min[2])/2,
	}
}

// floorCell 返回浮点坐标落在哪个 cell 宽的格子里，负数向下取整。
func floorCell(pos float64, cell int32) int32 {
	return int32(math.Floor(pos / float64(cell)))
}

// heightOffset 尽量保留源高度；需要时整体平移，使 384 格高的结构也能完整放入主世界。
func heightOffset(minY, maxY int32) (int32, error) {
	height := maxY - minY
	if height > 2096 {
		return 0, fmt.Errorf("structure: height %d exceeds supported Java height 2096", height)
	}
	if height > 384 {
		// 超过 384 格的结构靠数据包抬高世界高度，从主世界底部开始放。
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
