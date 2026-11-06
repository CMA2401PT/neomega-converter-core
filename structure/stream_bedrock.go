package structure

import (
	"context"
	"encoding/binary"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"celler"
	"celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converter "neomega-converter-core"
	"neomega-converter-core/internal/fileview"
	beexport "neomega-converter-core/je2be/export"
	rawnbt "neomega-nbt"
	goencode "neomega-nbt/encoder-go-object"
	nbt "neomega-nbt/presets/v1"
)

type layerStreams struct {
	dir     string
	current *numberStream
	layers  []*numberStream
}

func (l *layerStreams) GetReciver(rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	return goencode.ValueReciver{ListConstructor: l}, nil
}
func (l *layerStreams) HasFlag(rawnbt.Flag) bool { return false }
func (l *layerStreams) GetReusableElementPointer(t rawnbt.HighLevelValueType, length int, erase bool) (any, error) {
	l.current = &numberStream{dir: l.dir}
	return l.current, nil
}
func (l *layerStreams) OnNextElement() error {
	saved := *l.current
	l.layers = append(l.layers, &saved)
	*l.current = numberStream{dir: l.dir}
	return nil
}
func (l *layerStreams) OnListFinish() error { l.layers = append(l.layers, l.current); return nil }
func (l *layerStreams) NonTypedEmptyList() error {
	return fmt.Errorf("mcstructure: missing block_indices")
}
func (l *layerStreams) TypedEmptyList(rawnbt.HighLevelValueType) error { return l.NonTypedEmptyList() }

type bedrockPaletteEntry struct {
	Name    string         `nbt:"name"`
	States  map[string]any `nbt:"states"`
	Version int32          `nbt:"version"`
}
type nativeStructure struct {
	Format    int32    `nbt:"format_version"`
	Origin    [3]int32 `nbt:"structure_world_origin"`
	Size      [3]int32 `nbt:"size"`
	Structure struct {
		Indices  *layerStreams  `nbt:"block_indices"`
		Entities []nbt.Compound `nbt:"entities"`
		Palette  struct {
			Default struct {
				Blocks    []bedrockPaletteEntry `nbt:"block_palette"`
				Positions map[string]struct {
					Entity nbt.Compound `nbt:"block_entity_data"`
				} `nbt:"block_position_data"`
			} `nbt:"default"`
		} `nbt:"palette"`
	} `nbt:"structure"`
}

// convertMCStructure 保留基岩双层状态和条目版本，直接写基岩存档，不经过 Java 映射。
func convertMCStructure(ctx context.Context, path, output, name string, options Options, report func(Progress)) error {
	temp := filepath.Dir(output)
	h := &nativeStructure{}
	h.Structure.Indices = &layerStreams{dir: temp}
	defer func() {
		for _, s := range h.Structure.Indices.layers {
			if s.file != nil {
				s.file.Close()
			}
		}
		s := h.Structure.Indices.current
		if s != nil && s.file != nil {
			s.file.Close()
		}
	}()
	if err := readNBT(ctx, path, &streamCompound{target: h, children: map[string]any{"structure": &streamCompound{target: &h.Structure, children: map[string]any{"block_indices": h.Structure.Indices}}}}, nbt.EncodingLittleEndian, report); err != nil {
		return err
	}
	if h.Format != 1 {
		return fmt.Errorf("mcstructure: unsupported format_version %d", h.Format)
	}
	for _, n := range h.Size {
		if n <= 0 {
			return fmt.Errorf("mcstructure: invalid size")
		}
	}
	if h.Size[1] > 384 {
		return fmt.Errorf("mcstructure: height %d exceeds Bedrock overworld height 384", h.Size[1])
	}
	volume := int64(h.Size[0]) * int64(h.Size[1]) * int64(h.Size[2])
	offset, err := heightOffset(h.Origin[1], h.Origin[1]+h.Size[1])
	if err != nil {
		return err
	}
	h.Origin[1] += offset
	sink, err := newBlockSpool(ctx, temp)
	if err != nil {
		return err
	}
	defer sink.file.Close()
	entries := h.Structure.Palette.Default.Blocks
	palette := make([]bedrock.Block, len(entries))
	if len(palette) == 0 {
		return fmt.Errorf("mcstructure: empty palette")
	}
	for i, entry := range entries {
		state, err := ir.NewIRBlockFromGo(entry.Name, entry.States)
		if err != nil {
			return err
		}
		palette[i] = bedrock.Block{State: state.Tidy(), Version: entry.Version}
		sink.palette = append(sink.palette, state.Tidy())
	}
	sink.palette = append(sink.palette, sink.palette...)
	if len(h.Structure.Indices.layers) == 0 || len(h.Structure.Indices.layers) > 2 {
		return fmt.Errorf("mcstructure: invalid layer count")
	}
	for layer, stream := range h.Structure.Indices.layers {
		if stream.count != volume {
			return fmt.Errorf("mcstructure: layer length %d, expected %d", stream.count, volume)
		}
		index := uint64(0)
		err = stream.eachRun(func(value, count uint64) error {
			start := index
			index += count
			id := int64(value)
			if id == -1 {
				return nil
			}
			if id < 0 || id >= int64(len(palette)) {
				return fmt.Errorf("mcstructure: palette index %d", id)
			}
			if isAir(palette[id].State) {
				return nil
			}
			// 基岩体积为 X-Y-Z，连续游程沿 Z 前进。
			for count > 0 {
				z := start % uint64(h.Size[2])
				row := start / uint64(h.Size[2])
				y := row % uint64(h.Size[1])
				x := row / uint64(h.Size[1])
				n := min(count, uint64(h.Size[2])-z)
				for dz := uint64(0); dz < n; dz++ {
					if err := sink.run(h.Origin[0]+int32(x), h.Origin[1]+int32(y), h.Origin[2]+int32(z+dz), 1, uint32(id)+uint32(layer*len(palette)), false); err != nil {
						return err
					}
				}
				start += n
				count -= n
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	for key, data := range h.Structure.Palette.Default.Positions {
		if data.Entity == nil {
			continue
		}
		var index int64
		if _, err := fmt.Sscan(key, &index); err != nil || index < 0 || index >= volume {
			return fmt.Errorf("mcstructure: invalid block position %q", key)
		}
		z := int32(index % int64(h.Size[2]))
		row := index / int64(h.Size[2])
		y := int32(row % int64(h.Size[1]))
		x := int32(row / int64(h.Size[1]))
		value := maps.Clone(data.Entity)
		pos := celler.BlockPos{X: x + h.Origin[0], Y: y + h.Origin[1], Z: z + h.Origin[2]}
		value["x"], value["y"], value["z"] = pos.X, pos.Y, pos.Z
		sink.addTile(cellerjava.BlockEntity{Pos: pos, NBT: value})
	}
	for _, raw := range h.Structure.Entities {
		pos, ok := float3(raw["Pos"])
		if !ok {
			return fmt.Errorf("mcstructure: entity missing Pos")
		}
		value := maps.Clone(raw)
		value["Pos"] = []float32{float32(pos[0]) + float32(h.Origin[0]), float32(pos[1]) + float32(h.Origin[1]), float32(pos[2]) + float32(h.Origin[2])}
		sink.addEntity(value)
	}
	for _, b := range sink.buckets {
		if err := sink.flush(b); err != nil {
			return err
		}
	}
	if err := sink.writer.Flush(); err != nil {
		return err
	}
	view, err := fileview.Open(sink.file, max(0, int64(options.MemoryMiB-64)*1024*1024))
	if err != nil {
		return err
	}
	defer view.Close()
	sink.reader = view
	world, err := bedrock.Open(output, bedrock.Options{})
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			world.Close()
		}
	}()
	done := int64(0)
	report(Progress{Stage: "writing", Total: int64(len(sink.buckets))})
	actorID := int64(1)
	for pos, b := range sink.buckets {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunks := map[celler.ChunkPos]*converter.BedrockChunk{}
		chunkAt := func(p celler.ChunkPos) *converter.BedrockChunk {
			chunk := chunks[p]
			if chunk == nil {
				chunk = &converter.BedrockChunk{Pos: p, Sections: map[int32]*converter.BedrockSection{}}
				chunks[p] = chunk
			}
			return chunk
		}
		err = sink.readBucket(ctx, pos, b, func(x, y, z, length int32, state uint32) error {
			layer, index := int(state)/len(palette), int(state)%len(palette)
			for dx := int32(0); dx < length; dx++ {
				p := celler.ChunkPos{X: (x + dx) >> 4, Z: z >> 4}
				chunk := chunkAt(p)
				section := chunk.Sections[y>>4]
				if section == nil {
					section = &converter.BedrockSection{Layers: []*layer_storage.Palette[bedrock.Block]{layer_storage.MakePaletteStorage(bedrock.Block{State: air, Version: palette[index].Version}), layer_storage.MakePaletteStorage(bedrock.Block{State: air, Version: palette[index].Version})}}
					chunk.Sections[y>>4] = section
				}
				section.Layers[layer].Set(layer_storage.At(x+dx, y, z), palette[index])
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, tile := range b.tiles {
			chunk := chunkAt(tile.Pos.Chunk())
			chunk.BlockEntities = append(chunk.BlockEntities, tile.NBT)
		}
		for _, entity := range b.entities {
			p, _ := float3(entity["Pos"])
			chunk := chunkAt(celler.ChunkPos{X: int32(floorDiv(p[0], 16)), Z: int32(floorDiv(p[2], 16))})
			var key [8]byte
			binary.LittleEndian.PutUint64(key[:], uint64(actorID))
			entity["UniqueID"] = actorID
			actorID++
			chunk.Actors = append(chunk.Actors, converter.BedrockActor{ID: key, NBT: entity})
		}
		for _, chunk := range chunks {
			if err := beexport.WriteChunk(world, chunk); err != nil {
				return err
			}
		}
		done++
		report(Progress{Stage: "writing", Completed: done, Total: int64(len(sink.buckets))})
	}
	maximum := [3]int32{h.Origin[0] + h.Size[0], h.Origin[1] + h.Size[1], h.Origin[2] + h.Size[2]}
	spawn := spawnAbove(h.Origin, maximum)
	level := bedrock.Level{Version: 10, NBT: nbt.Document{Value: nbt.Compound{"StorageVersion": int32(10), "lastOpenedWithVersion": []int32{1, 21, 50, 0}, "LevelName": name, "SpawnX": spawn[0], "SpawnY": spawn[1], "SpawnZ": spawn[2], "GameType": int32(1), "Generator": int32(2), "FlatWorldLayers": `{"biome_id":1,"block_layers":[]}`}}}
	if err = world.WriteLevel(level); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "levelname.txt"), []byte(name), 0644); err != nil {
		return err
	}
	closed = true
	return world.Close()
}
