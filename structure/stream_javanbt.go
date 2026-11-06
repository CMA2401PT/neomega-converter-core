package structure

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"maps"
	"os"

	rawnbt "neomega-nbt"
	goencode "neomega-nbt/encoder-go-object"
	nbt "neomega-nbt/presets/v1"
)

// blockStream 复用同一个放置记录接收百万项 blocks 列表，只有方块实体保留独立 NBT。
type blockStream struct {
	dir, path        string
	file             *os.File
	zipped           *gzip.Writer
	compressedWriter *bufio.Writer
	writer           *bufio.Writer
	current          templateBlock
	tiles            []templateBlock
	count            int
}

func (b *blockStream) GetReciver(rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	return goencode.ValueReciver{ListConstructor: b}, nil
}
func (b *blockStream) HasFlag(rawnbt.Flag) bool { return false }
func (b *blockStream) GetReusableElementPointer(t rawnbt.HighLevelValueType, length int, erase bool) (any, error) {
	file, err := os.CreateTemp(b.dir, "placements-*")
	if err != nil {
		return nil, err
	}
	b.file, b.path = file, file.Name()
	b.compressedWriter = bufio.NewWriterSize(file, 64*1024)
	b.zipped, _ = gzip.NewWriterLevel(b.compressedWriter, gzip.BestSpeed)
	b.writer = bufio.NewWriterSize(b.zipped, 64*1024)
	b.count = length
	return &b.current, nil
}
func (b *blockStream) commit() error {
	var raw [16]byte
	for axis, p := range b.current.Pos {
		binary.LittleEndian.PutUint32(raw[axis*4:], uint32(p))
	}
	binary.LittleEndian.PutUint32(raw[12:], uint32(b.current.State))
	if _, err := b.writer.Write(raw[:]); err != nil {
		return err
	}
	if b.current.NBT != nil {
		b.tiles = append(b.tiles, b.current)
	}
	b.current = templateBlock{}
	return nil
}
func (b *blockStream) OnNextElement() error { return b.commit() }
func (b *blockStream) OnListFinish() error {
	if err := b.commit(); err != nil {
		return err
	}
	return b.finish()
}
func (b *blockStream) finish() error {
	if err := b.writer.Flush(); err != nil {
		return err
	}
	if err := b.zipped.Close(); err != nil {
		return err
	}
	if err := b.compressedWriter.Flush(); err != nil {
		return err
	}
	return b.file.Close()
}
func (b *blockStream) NonTypedEmptyList() error {
	_, err := b.GetReusableElementPointer(rawnbt.ComplexCompound, 0, false)
	if err != nil {
		return err
	}
	return b.finish()
}
func (b *blockStream) TypedEmptyList(t rawnbt.HighLevelValueType) error { return b.NonTypedEmptyList() }

func streamJavaNBTFile(path, temp string, sink *blockSpool) ([3]int32, error) {
	h := struct {
		Size     [3]int32             `nbt:"size"`
		Palette  []javaPaletteEntry   `nbt:"palette"`
		Palettes [][]javaPaletteEntry `nbt:"palettes"`
		Blocks   *blockStream         `nbt:"blocks"`
		Entities []templateEntity     `nbt:"entities"`
	}{Blocks: &blockStream{dir: temp}}
	defer func() {
		if h.Blocks.file != nil {
			h.Blocks.file.Close()
		}
	}()
	if err := readNBT(sink.ctx, path, &streamCompound{target: &h, children: map[string]any{"blocks": h.Blocks}}, nbt.EncodingBigEndian, sink.progress); err != nil {
		return [3]int32{}, err
	}
	for _, size := range h.Size {
		if size <= 0 {
			return [3]int32{}, fmt.Errorf("nbt: invalid size")
		}
	}
	if len(h.Palette) == 0 && len(h.Palettes) > 0 {
		h.Palette = h.Palettes[0]
	}
	var err error
	sink.palette, err = javaPalette(h.Palette)
	if err != nil {
		return [3]int32{}, err
	}
	offset, err := heightOffset(0, h.Size[1])
	if err != nil {
		return [3]int32{}, err
	}
	minimum := [3]int32{0, offset, 0}
	maximum := [3]int32{h.Size[0], h.Size[1] + offset, h.Size[2]}
	file, err := os.Open(h.Blocks.path)
	if err != nil {
		return [3]int32{}, err
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return [3]int32{}, err
	}
	defer zipped.Close()
	reader := bufio.NewReaderSize(zipped, 64*1024)
	var data [16]byte
	for index := 0; index < h.Blocks.count; index++ {
		if _, err = io.ReadFull(reader, data[:]); err != nil {
			return [3]int32{}, err
		}
		pos := [3]int32{}
		for axis := range 3 {
			pos[axis] = int32(binary.LittleEndian.Uint32(data[axis*4:]))
			if pos[axis] < 0 || pos[axis] >= h.Size[axis] {
				return [3]int32{}, fmt.Errorf("nbt: block %d outside size", index)
			}
		}
		state := binary.LittleEndian.Uint32(data[12:])
		if err = sink.run(pos[0], pos[1]+offset, pos[2], 1, state, false); err != nil {
			return [3]int32{}, err
		}
	}
	for _, tile := range h.Blocks.tiles {
		id, _ := tile.NBT["id"].(string)
		if id == "" {
			return [3]int32{}, fmt.Errorf("nbt: block entity missing id")
		}
		sink.addTile(javaBlockEntity(tile.Pos[0], tile.Pos[1]+offset, tile.Pos[2], id, tile.NBT))
	}
	for _, entity := range h.Entities {
		value := maps.Clone(entity.NBT)
		value["Pos"] = []float64{entity.Pos[0], entity.Pos[1] + float64(offset), entity.Pos[2]}
		sink.addEntity(value)
	}
	return spawnAbove(minimum, maximum), nil
}

// templateBlock 只暂存一次方块放置；实体 NBT 独立于方块数组保留。
type templateBlock struct {
	State int32        `nbt:"state"`
	Pos   [3]int32     `nbt:"pos"`
	NBT   nbt.Compound `nbt:"nbt"`
}
type templateEntity struct {
	Pos      [3]float64   `nbt:"pos"`
	BlockPos [3]int32     `nbt:"blockPos"`
	NBT      nbt.Compound `nbt:"nbt"`
}
