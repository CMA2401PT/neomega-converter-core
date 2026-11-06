package structure

import (
	"bufio"
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"slices"

	"celler/bedrock"
	cellerjava "celler/java"
	"neomega-blocks/ir"
	"neomega-converter-core/export"
	"neomega-converter-core/internal/fileview"
	"neomega-converter-core/internal/pipeline"
	nbt "neomega-nbt/presets/v1"
)

// 结构先按 128×128 的水平片区把游程落盘；写出时一个片区最多展开成 64 根区块柱。
const (
	bucketShift = 7
	bucketWidth = 1 << bucketShift
	// 片区记录每 64 KiB 落一块，解压时按同一粒度准备缓冲。
	bucketChunk = 64 * 1024
	// 一条游程记录的宽度：片区内的 x/z、世界 y 和调色板下标各占 4 字节。
	recordSize = 12
	// 所有片区累计的追加缓冲上限，超过就整体压缩落盘。
	bufferBudget = 8 * 1024 * 1024
)

type bucketPos struct{ x, z int32 }

// extent 是片区记录压缩后的位置与长度，raw 是解压后的大小。
type extent struct {
	offset int64
	size   int
	raw    int
}

// bucket 是一个水平片区：追加中的记录缓冲、已经落盘的分段、方块实体和实体。
type bucket struct {
	buffer   []byte
	extents  []extent
	tiles    []cellerjava.BlockEntity
	entities []nbt.Compound
}

// blockSpool 把源文件的扫描顺序转成小片区顺序。
// 游程只拆到 128×128 的水平片区边界；累计缓冲写满后压缩落盘。
// 写出任务各自展开一个片区，生成最多 64 根区块柱，不保留整张图的方块数组。
type blockSpool struct {
	ctx         context.Context
	progress    func(Progress)
	memoryMiB   int
	workers     int
	file        *os.File
	reader      *fileview.View
	buckets     map[bucketPos]*bucket
	palette     []ir.IRBlock
	bufferBytes int
	writer      *bufio.Writer
	offset      int64
	compressed  bytes.Buffer
	compressor  *flate.Writer
	highY       int32
}

// newBlockSpool 为本次转换建暂存文件；预算、并发和进度都取自 job。
func newBlockSpool(j *job) (*blockSpool, error) {
	file, err := os.CreateTemp(j.staging, "columns-*")
	if err != nil {
		return nil, err
	}
	s := &blockSpool{
		ctx:       j.ctx,
		progress:  j.report,
		memoryMiB: j.opts.MemoryMiB,
		workers:   j.opts.Workers,
		file:      file,
		buckets:   map[bucketPos]*bucket{},
		highY:     maxWorldY,
	}
	s.writer = bufio.NewWriterSize(file, 1024*1024)
	// BestSpeed 是合法级别，flate 不会在这里返回错误。
	s.compressor, _ = flate.NewWriter(&s.compressed, flate.BestSpeed)
	return s, nil
}

// at 取一个片区，没有就建一个空的。
func (s *blockSpool) at(pos bucketPos) *bucket {
	b := s.buckets[pos]
	if b == nil {
		b = &bucket{}
		s.buckets[pos] = b
	}
	return b
}

// run 写一段沿 X 连续的同状态方块。clearAir 用于后来的投影区域覆盖前面的区域。
func (s *blockSpool) run(x, y, z, length int32, state uint32, clearAir bool) error {
	if int(state) >= len(s.palette) || s.palette[state] == "" {
		return fmt.Errorf("structure: palette index %d", state)
	}
	if isAir(s.palette[state]) && !clearAir {
		return nil
	}
	s.highY = max(s.highY, (y>>4)*16+16)
	if err := s.ctx.Err(); err != nil {
		return err
	}
	for length > 0 {
		// 一段游程不跨片区边界，逐段写进各自片区的记录缓冲。
		n := min(length, bucketWidth-(x&(bucketWidth-1)))
		b := s.at(bucketPos{x >> bucketShift, z >> bucketShift})
		// 片区内 X/Z 各占 7 bit，世界 y 减主世界底部后存入剩余位，容纳扩展世界高度。
		start := uint32(x&(bucketWidth-1)) | uint32(z&(bucketWidth-1))<<bucketShift | uint32(y-minWorldY)<<(2*bucketShift)
		oldCapacity := cap(b.buffer)
		b.buffer = binary.LittleEndian.AppendUint32(b.buffer, start)
		b.buffer = binary.LittleEndian.AppendUint32(b.buffer, uint32(n))
		b.buffer = binary.LittleEndian.AppendUint32(b.buffer, state)
		s.bufferBytes += cap(b.buffer) - oldCapacity
		if len(b.buffer) >= bucketChunk-recordSize {
			if err := s.flush(b); err != nil {
				return err
			}
		}
		if s.bufferBytes >= bufferBudget {
			for _, pending := range s.buckets {
				if err := s.flush(pending); err != nil {
					return err
				}
			}
		}
		x += n
		length -= n
	}
	return nil
}

// flush 压缩一个片区的追加缓冲，记下分段位置后清空缓冲。
func (s *blockSpool) flush(b *bucket) error {
	if b.buffer == nil {
		return nil
	}
	s.compressed.Reset()
	s.compressor.Reset(&s.compressed)
	if _, err := s.compressor.Write(b.buffer); err != nil {
		return err
	}
	if err := s.compressor.Close(); err != nil {
		return err
	}
	data := s.compressed.Bytes()
	if _, err := s.writer.Write(data); err != nil {
		return err
	}
	b.extents = append(b.extents, extent{s.offset, len(data), len(b.buffer)})
	s.offset += int64(len(data))
	s.bufferBytes -= cap(b.buffer)
	b.buffer = nil
	return nil
}

// linearRun 将 Y-Z-X 扫描的一段方块按行拆开，不逐格计算除法与坐标。
// 空气和越界的调色板下标交给 run 处理，这里只负责换算。
func (s *blockSpool) linearRun(index, count uint64, size, origin [3]int32, state uint32, clearAir bool) error {
	width, length := uint64(size[0]), uint64(size[2])
	for count > 0 {
		x := index % width
		row := index / width
		z := row % length
		y := row / length
		n := min(count, width-x)
		if err := s.run(origin[0]+int32(x), origin[1]+int32(y), origin[2]+int32(z), int32(n), state, clearAir); err != nil {
			return err
		}
		index += n
		count -= n
	}
	return nil
}

// addTile 把方块实体挂到它所属的片区，写出时再按区块归类。
func (s *blockSpool) addTile(tile cellerjava.BlockEntity) {
	b := s.at(bucketPos{tile.Pos.X >> bucketShift, tile.Pos.Z >> bucketShift})
	b.tiles = append(b.tiles, tile)
}

// addEntity 把实体挂到它所在水平位置的片区，写出时再按区块归类。
func (s *blockSpool) addEntity(entity nbt.Compound) {
	pos, ok := nbtFloat3(entity["Pos"])
	if !ok {
		return
	}
	b := s.at(bucketPos{floorCell(pos[0], bucketWidth), floorCell(pos[2], bucketWidth)})
	b.entities = append(b.entities, entity)
}

// seal 结束暂存：刷出剩余游程，把记录文件映射成只读视图供 readBucket 读取。
// reserveBytes 留给写出阶段，不参与映射；预算不足或平台未实现映射时按偏移读取。
func (s *blockSpool) seal(reserveBytes int64) (*fileview.View, error) {
	for _, b := range s.buckets {
		if err := s.flush(b); err != nil {
			return nil, err
		}
	}
	if err := s.writer.Flush(); err != nil {
		return nil, err
	}
	view, err := fileview.Open(s.file, max(0, int64(s.memoryMiB)*1024*1024-reserveBytes))
	if err != nil {
		return nil, err
	}
	s.reader = view
	return view, nil
}

// discard 释放没有正常收尾的暂存文件。
func (s *blockSpool) discard() error {
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

// writeWorld 把 spool 里的方块写成 Java 存档，返回写出的区块数。
func (s *blockSpool) writeWorld(dir, name string, spawn [3]int32) (int, error) {
	// 按整个片区的高度预留调色板、编码结果和压缩工作区；这是并发估算，不是堆上限。
	workerBytes := max(int64(32*1024*1024), bucketWidth*bucketWidth*int64(s.highY-minWorldY)*4)
	view, err := s.seal(workerBytes)
	if err != nil {
		return 0, err
	}
	defer view.Close()
	world, err := cellerjava.OpenDirectory(dir, cellerjava.Options{})
	if err != nil {
		return 0, err
	}
	defer world.Close()
	positions := make([]bucketPos, 0, len(s.buckets))
	for pos := range s.buckets {
		positions = append(positions, pos)
	}
	slices.SortFunc(positions, func(a, b bucketPos) int {
		if a.z != b.z {
			return int(a.z - b.z)
		}
		return int(a.x - b.x)
	})
	workers, err := pipeline.Workers(pipeline.Limit{
		MemoryMiB: s.memoryMiB,
		Requested: s.workers,
		Reserved:  view.MappedBytes,
		TaskBytes: workerBytes,
	})
	if err != nil {
		return 0, err
	}
	count, err := s.writeBuckets(world, positions, workers)
	if err != nil {
		return count, err
	}
	level, err := javaLevel(dir, name, spawn, s.highY)
	if err != nil {
		return count, err
	}
	if err = world.WriteLevel(level); err != nil {
		return count, err
	}
	return count, world.Commit()
}

// javaLevel 生成这次转换的 level.dat：空超平坦主世界，出生点就是结构顶面。
// 高出主世界的结构再写一份扩展高度的数据包。
func javaLevel(dir, name string, spawn [3]int32, highY int32) (nbt.Document, error) {
	level, _, err := export.ConvertLevel(bedrock.Level{NBT: nbt.Document{Value: nbt.Compound{
		"LevelName": name, "SpawnX": spawn[0], "SpawnY": spawn[1], "SpawnZ": spawn[2],
		"GameType": int32(1), "commandsAllowed": int8(1), "Generator": int32(2), "FlatWorldLayers": `{"biome_id":1,"block_layers":[]}`,
	}}}, export.DataVersion)
	if err != nil {
		return nbt.Document{}, err
	}
	if highY > maxWorldY {
		if err := writeHeightPack(dir, &level, highY); err != nil {
			return nbt.Document{}, err
		}
	}
	return level, nil
}

// readBucket 依次解压一个片区的记录，Java 与 Bedrock 共用空间重排结果。
func (s *blockSpool) readBucket(ctx context.Context, p bucketPos, b *bucket, visit func(x, y, z, length int32, state uint32) error) error {
	raw := make([]byte, bucketChunk)
	compressed := make([]byte, 2*bucketChunk)
	reader := flate.NewReader(bytes.NewReader(nil))
	defer reader.Close()
	for _, part := range b.extents {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := s.reader.ReadRange(part.offset, part.size, compressed)
		if err != nil {
			return err
		}
		if err := reader.(flate.Resetter).Reset(bytes.NewReader(data), nil); err != nil {
			return err
		}
		if _, err = io.ReadFull(reader, raw[:part.raw]); err != nil {
			return err
		}
		for data := raw[:part.raw]; len(data) > 0; data = data[recordSize:] {
			start := binary.LittleEndian.Uint32(data)
			length := binary.LittleEndian.Uint32(data[4:])
			state := binary.LittleEndian.Uint32(data[8:])
			x := p.x*bucketWidth + int32(start&(bucketWidth-1))
			y := int32(start>>(2*bucketShift)) + minWorldY
			z := p.z*bucketWidth + int32(start>>bucketShift&(bucketWidth-1))
			if err := visit(x, y, z, int32(length), state); err != nil {
				return err
			}
		}
	}
	return nil
}

// clearTiles 让后一个投影区域的方块覆盖时同时移除旧区域的方块实体。
func (s *blockSpool) clearTiles(minimum, size [3]int32) {
	for _, b := range s.buckets {
		kept := b.tiles[:0]
		for _, tile := range b.tiles {
			p := tile.Pos
			inside := p.X >= minimum[0] && p.X < minimum[0]+size[0] &&
				p.Y >= minimum[1] && p.Y < minimum[1]+size[1] &&
				p.Z >= minimum[2] && p.Z < minimum[2]+size[2]
			if inside {
				continue
			}
			kept = append(kept, tile)
		}
		b.tiles = kept
	}
}
