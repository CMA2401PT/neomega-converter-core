package worldinput

import (
	"celler"
	"celler/java"
	"errors"
	"io/fs"
	"path"
	"slices"
)

// javaDimensions 只扫描目录，避免维度选择触发全世界解码。
func javaDimensions(files fs.FS, selected string) (java.Dimension, []string, error) {
	dimensions := []string{}
	for _, name := range []string{".", "DIM-1", "DIM1"} {
		if _, err := fs.Stat(files, path.Join(name, "region")); err == nil {
			dimensions = append(dimensions, name)
		}
	}
	err := fs.WalkDir(files, "dimensions", func(name string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if entry.Name() == "region" {
			dimensions = append(dimensions, path.Dir(name))
			return fs.SkipDir
		}
		if entry.Name() == "entities" || entry.Name() == "poi" {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	if selected == "" {
		selected = "."
		if !slices.Contains(dimensions, ".") && slices.Contains(dimensions, "dimensions/minecraft/overworld") {
			selected = "dimensions/minecraft/overworld"
		}
	}
	if !fs.ValidPath(selected) {
		return "", nil, fs.ErrInvalid
	}
	if !slices.Contains(dimensions, selected) {
		dimensions = append(dimensions, selected)
	}
	return java.Dimension(selected), dimensions, nil
}

// javaPosition 优先采用出生点，所选维度在那里没有区块时回退到已有内容。
func javaPosition(world *java.World, dimension java.Dimension) ([3]float64, error) {
	position := [3]float64{0, 80, 0}
	if level, err := world.ReadLevel(); err == nil {
		if data, ok := level.Value["Data"].(map[string]any); ok {
			for axis, key := range []string{"SpawnX", "SpawnY", "SpawnZ"} {
				if value, ok := data[key].(int32); ok {
					position[axis] = float64(value)
				}
			}
			position[1] += 12
		}
	}
	// Find one occupied slot if the selected dimension has no chunk at spawn.
	p := celler.ChunkPos{
		X: int32(position[0]) >> 4,
		Z: int32(position[2]) >> 4,
	}
	if _, err := world.ReadRaw(dimension, java.Terrain, p); errors.Is(err, fs.ErrNotExist) {
		found := errors.New("found first chunk")
		err = world.Scan(dimension, java.Terrain, func(pos celler.ChunkPos) error {
			position[0] = float64(pos.X)*16 + 8
			position[2] = float64(pos.Z)*16 + 8
			if c, e := world.ReadChunk(dimension, pos); e == nil {
				if ys, e := c.SectionYs(); e == nil {
					highest := int32(-129)
					for _, y := range ys {
						section, e := c.Section(y)
						if e != nil || section.Blocks == nil {
							continue
						}
						for _, block := range section.Blocks.GetUsedPalette() {
							switch block.NameWithNameSpace() {
							case "minecraft:air", "minecraft:cave_air", "minecraft:void_air":
							default:
								highest = max(highest, y)
							}
						}
					}
					if highest >= -128 {
						position[1] = float64(highest*16 + 24)
					}
				}
			}
			return found
		})
		if err != nil && !errors.Is(err, found) {
			return position, err
		}
	} else if err != nil {
		return position, err
	}
	return position, nil
}
