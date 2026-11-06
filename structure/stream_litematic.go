package structure

import (
	"fmt"
	"math/bits"
	"neomega-blocks/ir"
	rawnbt "neomega-nbt"
	goencode "neomega-nbt/encoder-go-object"
	nbt "neomega-nbt/presets/v1"
	"slices"
)

type javaPaletteEntry struct {
	Name       string
	Properties map[string]any
}

func javaPalette(entries []javaPaletteEntry) ([]ir.IRBlock, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("structure: empty Java palette")
	}
	result := make([]ir.IRBlock, len(entries))
	for i, entry := range entries {
		block, err := ir.NewIRBlockFromGo(entry.Name, entry.Properties)
		if err != nil {
			return nil, err
		}
		result[i] = block.Tidy()
	}
	return result, nil
}

type vector struct {
	X int32 `nbt:"x"`
	Y int32 `nbt:"y"`
	Z int32 `nbt:"z"`
}
type streamRegion struct {
	Position          vector
	Size              vector
	BlockStatePalette []javaPaletteEntry
	BlockStates       *numberStream
	TileEntities      []nbt.Compound
	Entities          []nbt.Compound
}
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
func (r *regionMap) GetTagPointer(name string, t rawnbt.HighLevelValueType) (any, error) {
	region := &streamRegion{BlockStates: &numberStream{dir: r.dir}}
	r.regions[name] = region
	return &streamCompound{target: region, children: map[string]any{"BlockStates": region.BlockStates}}, nil
}
func regionBounds(r *streamRegion) (minimum, size [3]int32) {
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

func streamLitematicFile(path, temp string, sink *blockSpool) ([3]int32, error) {
	h := struct {
		Version int32
		Regions *regionMap
	}{Regions: &regionMap{dir: temp}}
	defer func() {
		for _, r := range h.Regions.regions {
			if r.BlockStates.file != nil {
				r.BlockStates.file.Close()
			}
		}
	}()
	if err := readNBT(sink.ctx, path, &streamCompound{target: &h, children: map[string]any{"Regions": h.Regions}}, nbt.EncodingBigEndian, sink.progress); err != nil {
		return [3]int32{}, err
	}
	if h.Version < 4 || h.Version > 7 {
		return [3]int32{}, fmt.Errorf("litematic: unsupported version %d", h.Version)
	}
	names := make([]string, 0, len(h.Regions.regions))
	for name := range h.Regions.regions {
		names = append(names, name)
	}
	slices.Sort(names)
	if len(names) == 0 {
		return [3]int32{}, fmt.Errorf("litematic: no regions")
	}
	minimum, size := regionBounds(h.Regions.regions[names[0]])
	maximum := [3]int32{minimum[0] + size[0], minimum[1] + size[1], minimum[2] + size[2]}
	for _, name := range names {
		low, size := regionBounds(h.Regions.regions[name])
		for axis := range 3 {
			if size[axis] <= 0 {
				return [3]int32{}, fmt.Errorf("litematic: invalid region size")
			}
			minimum[axis] = min(minimum[axis], low[axis])
			maximum[axis] = max(maximum[axis], low[axis]+size[axis])
		}
	}
	offset, err := heightOffset(minimum[1], maximum[1])
	if err != nil {
		return [3]int32{}, err
	}
	minimum[1] += offset
	maximum[1] += offset
	for regionIndex, name := range names {
		r := h.Regions.regions[name]
		low, size := regionBounds(r)
		low[1] += offset
		if regionIndex > 0 {
			sink.clearTiles(low, size)
		}
		palette, err := javaPalette(r.BlockStatePalette)
		if err != nil {
			return [3]int32{}, err
		}
		base := uint32(len(sink.palette))
		sink.palette = append(sink.palette, palette...)
		volume := uint64(size[0]) * uint64(size[1]) * uint64(size[2])
		width := max(2, bits.Len(uint(len(palette)-1)))
		spanWords := (volume*uint64(width) + 63) / 64
		perWord := 64 / width
		packedWords := (volume + uint64(perWord) - 1) / uint64(perWord)
		words := uint64(r.BlockStates.count)
		if words != spanWords && words != packedWords && !(len(palette) == 1 && words == 0) {
			return [3]int32{}, fmt.Errorf("litematic: region %q invalid BlockStates length", name)
		}
		reader, err := r.BlockStates.open()
		if err != nil {
			return [3]int32{}, err
		}
		err = func() error {
			defer reader.close()
			var word uint64
			remaining := uint(0)
			mask := uint64(1<<width) - 1
			var runValue uint32
			var runStart, runLength uint64
			flush := func() error {
				if runLength == 0 {
					return nil
				}
				return sink.linearRun(runStart, runLength, size, low, base+runValue, regionIndex > 0)
			}
			for index := uint64(0); index < volume; index++ {
				value := uint64(0)
				if words != 0 {
					if remaining == 0 {
						var err error
						word, err = reader.one()
						if err != nil {
							return err
						}
						remaining = 64
					}
					value = word & mask
					if remaining < uint(width) {
						tail := remaining
						next, err := reader.one()
						if err != nil {
							return err
						}
						value = (word | next<<tail) & mask
						word = next >> (uint(width) - tail)
						remaining = 64 - (uint(width) - tail)
					} else {
						word >>= uint(width)
						remaining -= uint(width)
					}
					if words != spanWords && remaining < uint(width) {
						remaining = 0
					}
				}
				if value >= uint64(len(palette)) {
					return fmt.Errorf("litematic: palette index %d", value)
				}
				if runLength > 0 && runValue != uint32(value) {
					if err := flush(); err != nil {
						return err
					}
					runLength = 0
				}
				if runLength == 0 {
					runStart, runValue = index, uint32(value)
				}
				runLength++
			}
			return flush()
		}()
		if err != nil {
			return [3]int32{}, err
		}
		for _, tile := range r.TileEntities {
			x, okX := nbtInt32(tile["x"])
			y, okY := nbtInt32(tile["y"])
			z, okZ := nbtInt32(tile["z"])
			id, _ := tile["id"].(string)
			if !okX || !okY || !okZ || id == "" {
				return [3]int32{}, fmt.Errorf("litematic: invalid tile entity")
			}
			sink.addTile(javaBlockEntity(low[0]+x, low[1]+y, low[2]+z, id, tile))
		}
		for _, raw := range r.Entities {
			if entity, ok := shiftEntity(raw, [3]int32{r.Position.X, r.Position.Y + offset, r.Position.Z}); ok {
				sink.addEntity(entity)
			}
		}
	}
	return spawnAbove(minimum, maximum), nil
}
