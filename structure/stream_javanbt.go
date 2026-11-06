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

// placementStream 把结构模板的 blocks 列表按固定记录写进临时文件。
// 一条记录 16 字节：三个 int32 坐标加一个 int32 调色板下标；带 NBT 的方块实体另留在内存里。
type placementStream struct {
	dir     string
	temp    *tempWriter
	current templateBlock
	tiles   []templateBlock
	count   int
}

func (p *placementStream) GetReciver(rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	return goencode.ValueReciver{ListConstructor: p}, nil
}
func (p *placementStream) HasFlag(rawnbt.Flag) bool { return false }

// GetReusableElementPointer 为整份列表开一个临时文件，返回复用的当前条目指针。
func (p *placementStream) GetReusableElementPointer(_ rawnbt.HighLevelValueType, length int, _ bool) (any, error) {
	temp, err := newTempWriter(p.dir, "placements-*")
	if err != nil {
		return nil, err
	}
	p.temp, p.count = temp, length
	return &p.current, nil
}

// commit 写出当前条目；带 NBT 的条目同时记进 tiles，其余立刻丢弃。
func (p *placementStream) commit() error {
	var raw [16]byte
	for axis, pos := range p.current.Pos {
		binary.LittleEndian.PutUint32(raw[axis*4:], uint32(pos))
	}
	binary.LittleEndian.PutUint32(raw[12:], uint32(p.current.State))
	if _, err := p.temp.records.Write(raw[:]); err != nil {
		return err
	}
	if p.current.NBT != nil {
		p.tiles = append(p.tiles, p.current)
	}
	p.current = templateBlock{}
	return nil
}

func (p *placementStream) OnNextElement() error { return p.commit() }
func (p *placementStream) OnListFinish() error {
	if err := p.commit(); err != nil {
		return err
	}
	return p.finish()
}
func (p *placementStream) finish() error { return p.temp.close() }
func (p *placementStream) NonTypedEmptyList() error {
	if _, err := p.GetReusableElementPointer(rawnbt.ComplexCompound, 0, false); err != nil {
		return err
	}
	return p.finish()
}
func (p *placementStream) TypedEmptyList(t rawnbt.HighLevelValueType) error { return p.NonTypedEmptyList() }

// discard 释放没有正常收尾的临时文件；列表缺失时 p.temp 为空。
func (p *placementStream) discard() error {
	if p.temp == nil {
		return nil
	}
	return p.temp.discard()
}

// open 读出刚写完的放置记录。
func (p *placementStream) open() (*placementReader, error) {
	// 文件里没有 blocks 列表时 temp 为空；报错而不是读出零条放置。
	if p.temp == nil {
		return nil, fmt.Errorf("nbt: missing blocks array")
	}
	file, err := os.Open(p.temp.path)
	if err != nil {
		return nil, err
	}
	zipped, err := gzip.NewReader(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	return &placementReader{file: file, zipped: zipped, reader: bufio.NewReaderSize(zipped, 64*1024)}, nil
}

type placementReader struct {
	file   *os.File
	zipped *gzip.Reader
	reader *bufio.Reader
}

// next 读出一条放置记录，坐标范围由调用方按结构尺寸检查。
func (r *placementReader) next() ([3]int32, uint32, error) {
	var raw [16]byte
	if _, err := io.ReadFull(r.reader, raw[:]); err != nil {
		return [3]int32{}, 0, err
	}
	pos := [3]int32{}
	for axis := range 3 {
		pos[axis] = int32(binary.LittleEndian.Uint32(raw[axis*4:]))
	}
	return pos, binary.LittleEndian.Uint32(raw[12:]), nil
}

func (r *placementReader) close() { r.zipped.Close(); r.file.Close() }

// templateFile 是原版结构模板（.nbt）的根标签。
type templateFile struct {
	Size     [3]int32             `nbt:"size"`
	Palette  []javaPaletteEntry   `nbt:"palette"`
	Palettes [][]javaPaletteEntry `nbt:"palettes"`
	Blocks   *placementStream     `nbt:"blocks"`
	Entities []templateEntity     `nbt:"entities"`
}

// convertTemplate 把原版结构模板写成 Java 存档。模板的方块只有一层，也没有方块实体层。
func convertTemplate(j *job) error {
	h := &templateFile{Blocks: &placementStream{dir: j.staging}}
	defer h.Blocks.discard()
	root := &streamCompound{target: h, children: map[string]any{"blocks": h.Blocks}}
	if err := readNBT(j.ctx, j.input, root, nbt.EncodingBigEndian, j.report); err != nil {
		return err
	}
	for _, size := range h.Size {
		if size <= 0 {
			return fmt.Errorf("nbt: invalid size")
		}
	}
	// 多调色板模板（palettes）按第一个调色板写出。
	if len(h.Palette) == 0 && len(h.Palettes) > 0 {
		h.Palette = h.Palettes[0]
	}
	var err error
	j.spool.palette, err = javaPalette(h.Palette)
	if err != nil {
		return err
	}
	offset, err := heightOffset(0, h.Size[1])
	if err != nil {
		return err
	}
	minimum := [3]int32{0, offset, 0}
	maximum := [3]int32{h.Size[0], h.Size[1] + offset, h.Size[2]}
	placements, err := h.Blocks.open()
	if err != nil {
		return err
	}
	defer placements.close()
	for index := 0; index < h.Blocks.count; index++ {
		pos, state, err := placements.next()
		if err != nil {
			return err
		}
		for axis := range 3 {
			if pos[axis] < 0 || pos[axis] >= h.Size[axis] {
				return fmt.Errorf("nbt: block %d outside size", index)
			}
		}
		if err = j.spool.run(pos[0], pos[1]+offset, pos[2], 1, state, false); err != nil {
			return err
		}
	}
	for _, tile := range h.Blocks.tiles {
		id := nbtID(tile.NBT)
		if id == "" {
			return fmt.Errorf("nbt: block entity missing id")
		}
		j.spool.addTile(javaBlockEntity(tile.Pos[0], tile.Pos[1]+offset, tile.Pos[2], id, tile.NBT))
	}
	for _, entity := range h.Entities {
		value := maps.Clone(entity.NBT)
		value["Pos"] = []float64{entity.Pos[0], entity.Pos[1] + float64(offset), entity.Pos[2]}
		j.spool.addEntity(value)
	}
	j.spawn = spawnPoint(minimum, maximum)
	return nil
}

// templateBlock 只暂存一次方块放置；方块实体 NBT 独立于方块数组保留。
type templateBlock struct {
	State int32        `nbt:"state"`
	Pos   [3]int32     `nbt:"pos"`
	NBT   nbt.Compound `nbt:"nbt"`
}

// templateEntity 是模板实体条目；blockPos 不参与写出，由丢弃接收器处理。
type templateEntity struct {
	Pos [3]float64   `nbt:"pos"`
	NBT nbt.Compound `nbt:"nbt"`
}
