package structure

import (
	"fmt"
	"maps"
	"math/bits"
	"slices"

	rawnbt "neomega-nbt"
	goencode "neomega-nbt/encoder-go-object"
	nbt "neomega-nbt/presets/v1"
)

// vector 是 NBT 里的三字段整数坐标。
type vector struct {
	X int32 `nbt:"x"`
	Y int32 `nbt:"y"`
	Z int32 `nbt:"z"`
}

// streamRegion 是一个 litematic 区域。Size 可以为负，表示区域从 Position 往负方向展开。
type streamRegion struct {
	Position          vector
	Size              vector
	BlockStatePalette []javaPaletteEntry
	BlockStates       *numberStream
	TileEntities      []nbt.Compound
	Entities          []nbt.Compound
}

// regionMap 接收 Regions 下按名字排列的区域；每个区域自己带一份方块数组。
type regionMap struct {
	dir     string
	regions map[string]*streamRegion
}

func (r *regionMap) GetReciver(rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	return goencode.ValueReciver{CompoundConstructor: r}, nil
}
func (r *regionMap) OnResetCompound() error  { r.regions = make(map[string]*streamRegion); return nil }
func (r *regionMap) OnNextElement() error    { return nil }
func (r *regionMap) OnCompoundFinish() error { return nil }
func (r *regionMap) GetTagPointer(name string, _ rawnbt.HighLevelValueType) (any, error) {
	region := &streamRegion{BlockStates: &numberStream{dir: r.dir}}
	r.regions[name] = region
	return &streamCompound{target: region, children: map[string]any{"BlockStates": region.BlockStates}}, nil
}

// bounds 返回区域的最小角与尺寸；负尺寸把最小角挪到负方向那一端。
func (r *streamRegion) bounds() (minimum, size [3]int32) {
	minimum = [3]int32{r.Position.X, r.Position.Y, r.Position.Z}
	size = [3]int32{r.Size.X, r.Size.Y, r.Size.Z}
	for axis := range 3 {
		if size[axis] < 0 {
			minimum[axis] += size[axis] + 1
			size[axis] = -size[axis]
		}
	}
	return
}

// packedLayout 是 BlockStates 的位打包方式：老版本每个 long 从边界重新开始，
// 新版本是连续位流，一个索引可以跨两个 long。
type packedLayout int

const (
	spanningLayout packedLayout = iota // 连续位流
	wordLayout                         // 每个 long 各自打包，尾部补齐
)

// eachRun 解出区域的全部方块，把连续相同的调色板索引合并成游程后交给 visit。
// volume 是区域体积，width 是每个索引占用的位数。
func (r *streamRegion) eachRun(volume uint64, width uint, layout packedLayout, visit func(start, length uint64, state uint32) error) error {
	reader, err := r.BlockStates.open()
	if err != nil {
		return err
	}
	defer reader.close()
	if r.BlockStates.count == 0 {
		// 单条目调色板的区域不写 BlockStates 数组，整片区域都是 0 号方块。
		return visit(0, volume, 0)
	}
	mask := uint64(1<<width) - 1
	var word uint64
	remaining := uint(0)
	var runState uint32
	var runStart, runLength uint64
	flush := func() error {
		if runLength == 0 {
			return nil
		}
		return visit(runStart, runLength, runState)
	}
	for index := uint64(0); index < volume; index++ {
		if remaining == 0 {
			next, err := reader.one()
			if err != nil {
				return err
			}
			word, remaining = next, 64
		}
		value := word & mask
		if remaining < width {
			// 当前 long 剩下的位不够一个索引，接上后一个 long 的高位。
			tail := remaining
			next, err := reader.one()
			if err != nil {
				return err
			}
			value = (word | next<<tail) & mask
			word, remaining = next>>(width-tail), 64-(width-tail)
		} else {
			word >>= width
			remaining -= width
		}
		if layout == wordLayout && remaining < width {
			remaining = 0 // 丢弃这个 long 尾部的填充位
		}
		if runLength > 0 && runState != uint32(value) {
			if err := flush(); err != nil {
				return err
			}
			runLength = 0
		}
		if runLength == 0 {
			runStart, runState = index, uint32(value)
		}
		runLength++
	}
	return flush()
}

// streamLitematic 是 litematic 的根标签。一个文件可以有多个命名区域，按名字排序后依次铺开。
type streamLitematic struct {
	Version int32
	Regions *regionMap
}

// convertLitematic 把 litematic 写成 Java 存档。后铺的区域覆盖先铺的：同范围的方块实体要清掉，
// 空气也要真正写下去。
func convertLitematic(j *job) error {
	h := &streamLitematic{Regions: &regionMap{dir: j.staging}}
	defer func() {
		for _, region := range h.Regions.regions {
			region.BlockStates.discard()
		}
	}()
	root := &streamCompound{target: h, children: map[string]any{"Regions": h.Regions}}
	if err := readNBT(j.ctx, j.input, root, nbt.EncodingBigEndian, j.report); err != nil {
		return err
	}
	if h.Version < 4 || h.Version > 7 {
		return fmt.Errorf("litematic: unsupported version %d", h.Version)
	}
	names := slices.Sorted(maps.Keys(h.Regions.regions))
	if len(names) == 0 {
		return fmt.Errorf("litematic: no regions")
	}
	minimum, extent := h.Regions.regions[names[0]].bounds()
	maximum := [3]int32{minimum[0] + extent[0], minimum[1] + extent[1], minimum[2] + extent[2]}
	for _, name := range names {
		low, size := h.Regions.regions[name].bounds()
		for axis := range 3 {
			if size[axis] <= 0 {
				return fmt.Errorf("litematic: invalid region size")
			}
			minimum[axis] = min(minimum[axis], low[axis])
			maximum[axis] = max(maximum[axis], low[axis]+size[axis])
		}
	}
	offset, err := heightOffset(minimum[1], maximum[1])
	if err != nil {
		return err
	}
	minimum[1] += offset
	maximum[1] += offset
	for position, name := range names {
		region := h.Regions.regions[name]
		low, size := region.bounds()
		low[1] += offset
		overlapping := position > 0
		if overlapping {
			j.spool.clearTiles(low, size)
		}
		palette, err := javaPalette(region.BlockStatePalette)
		if err != nil {
			return err
		}
		base := uint32(len(j.spool.palette))
		j.spool.palette = append(j.spool.palette, palette...)
		volume := uint64(size[0]) * uint64(size[1]) * uint64(size[2])
		width := uint(max(2, bits.Len(uint(len(palette)-1))))
		// 两种打包方式写入的 long 数不同：连续位流按位数算，逐 long 打包按每 long 能放几个索引算。
		spanningWords := (volume*uint64(width) + 63) / 64
		packedWords := (volume + uint64(64/width) - 1) / uint64(64/width)
		words := uint64(region.BlockStates.count)
		if words != spanningWords && words != packedWords && !(len(palette) == 1 && words == 0) {
			return fmt.Errorf("litematic: region %q invalid BlockStates length", name)
		}
		layout := wordLayout
		if words == spanningWords {
			layout = spanningLayout
		}
		err = region.eachRun(volume, width, layout, func(start, length uint64, state uint32) error {
			if int(state) >= len(palette) {
				return fmt.Errorf("litematic: palette index %d", state)
			}
			return j.spool.linearRun(start, length, size, low, base+state, overlapping)
		})
		if err != nil {
			return err
		}
		for _, tile := range region.TileEntities {
			pos, ok := tilePosition(tile)
			id := nbtID(tile)
			if !ok || id == "" {
				return fmt.Errorf("litematic: invalid tile entity")
			}
			j.spool.addTile(javaBlockEntity(low[0]+pos[0], low[1]+pos[1], low[2]+pos[2], id, tile))
		}
		// 实体坐标相对区域原点，负尺寸时原点不是最小角。
		origin := [3]int32{region.Position.X, region.Position.Y + offset, region.Position.Z}
		for _, raw := range region.Entities {
			if entity, ok := shiftEntity(raw, origin); ok {
				j.spool.addEntity(entity)
			}
		}
	}
	j.spawn = spawnPoint(minimum, maximum)
	return nil
}
