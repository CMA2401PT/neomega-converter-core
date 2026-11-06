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
	"runtime"
	"slices"

	"celler/bedrock"
	cellerjava "celler/java"
	"neomega-blocks/ir"
	"neomega-converter-core/export"
	"neomega-converter-core/internal/fileview"
	nbt "neomega-nbt/presets/v1"
)

type bucketPos struct{ x, z int32 }
type extent struct {
	offset int64
	size   int
	raw    int
}
type bucket struct {
	buffer   []byte
	extents  []extent
	tiles    []cellerjava.BlockEntity
	entities []nbt.Compound
}

// blockSpool 把源文件的扫描顺序转成小片区顺序。
// 游程只拆到 128×128 的水平片区边界；8 MiB 缓冲写满后压缩落盘。
// 写出任务各自展开一个片区，生成最多 64 根区块柱，不保留整张图的方块数组。
type blockSpool struct {
	reader      *fileview.View
	memoryBytes int64
	workers     int
	progress    func(Progress)
	ctx         context.Context
	file        *os.File
	buckets     map[bucketPos]*bucket
	palette     []ir.IRBlock
	bufferBytes int
	writer      *bufio.Writer
	offset      int64
	compressed  bytes.Buffer
	compressor  *flate.Writer
	highY       int32
}

func newBlockSpool(ctx context.Context, dir string) (*blockSpool, error) {
	file, err := os.CreateTemp(dir, "columns-*")
	if err != nil {
		return nil, err
	}
	s := &blockSpool{memoryBytes: 512 * 1024 * 1024, progress: func(Progress) {}, ctx: ctx, file: file, highY: 320, buckets: make(map[bucketPos]*bucket)}
	s.writer = bufio.NewWriterSize(file, 1024*1024)
	s.compressor, _ = flate.NewWriter(&s.compressed, flate.BestSpeed)
	return s, nil
}
func (s *blockSpool) bucket(pos bucketPos) *bucket {
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
		n := min(length, 128-(x&127))
		b := s.bucket(bucketPos{x >> 7, z >> 7})

		// 片区内 X/Z 各占 7 bit，Y 加 64 后存入剩余位，容纳扩展世界高度。
		start := uint32(x&127) | uint32(z&127)<<7 | uint32(y+64)<<14
		oldCapacity := cap(b.buffer)
		b.buffer = binary.LittleEndian.AppendUint32(b.buffer, start)
		b.buffer = binary.LittleEndian.AppendUint32(b.buffer, uint32(n))
		b.buffer = binary.LittleEndian.AppendUint32(b.buffer, state)
		s.bufferBytes += cap(b.buffer) - oldCapacity
		if len(b.buffer) >= 64*1024-12 {
			if err := s.flush(b); err != nil {
				return err
			}
		}
		if s.bufferBytes >= 8*1024*1024 {
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
func (s *blockSpool) linearRun(index, count uint64, size, origin [3]int32, state uint32, clearAir bool) error {
	if int(state) >= len(s.palette) || s.palette[state] == "" {
		return fmt.Errorf("structure: palette index %d", state)
	}
	if isAir(s.palette[state]) && !clearAir {
		return nil
	}
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
func (s *blockSpool) addTile(tile cellerjava.BlockEntity) {
	b := s.bucket(bucketPos{tile.Pos.X >> 7, tile.Pos.Z >> 7})
	b.tiles = append(b.tiles, tile)
}
func (s *blockSpool) addEntity(entity nbt.Compound) {
	pos, ok := float3(entity["Pos"])
	if !ok {
		return
	}
	b := s.bucket(bucketPos{int32(floorDiv(pos[0], 128)), int32(floorDiv(pos[2], 128))})
	b.entities = append(b.entities, entity)
}

func (s *blockSpool) write(dir, name string, spawn [3]int32) (int, error) {
	for _, b := range s.buckets {
		if err := s.flush(b); err != nil {
			return 0, err
		}
	}
	if err := s.writer.Flush(); err != nil {
		return 0, err
	}
	// 按整个片区的高度预留调色板、编码结果和压缩工作区；这是并发估算，非堆上限。
	workerBytes := max(int64(32*1024*1024), 128*128*int64(s.highY+64)*4)
	view, err := fileview.Open(s.file, max(0, s.memoryBytes-workerBytes))
	if err != nil {
		return 0, err
	}
	defer view.Close()
	s.reader = view
	world, err := cellerjava.OpenDirectory(dir, cellerjava.Options{})
	if err != nil {
		return 0, err
	}
	defer world.Close()
	positions := make([]bucketPos, 0, len(s.buckets))
	for p := range s.buckets {
		positions = append(positions, p)
	}
	slices.SortFunc(positions, func(a, b bucketPos) int {
		if a.z != b.z {
			return int(a.z - b.z)
		}
		return int(a.x - b.x)
	})
	workers := s.workers
	if workers == 0 {
		workers = min(runtime.GOMAXPROCS(0), 8)
	}
	workers = min(workers, max(1, int((s.memoryBytes-view.MappedBytes)/workerBytes)))
	count, err := s.writeBuckets(world, positions, workers)
	if err != nil {
		return count, err
	}
	level, _, err := export.ConvertLevel(bedrock.Level{NBT: nbt.Document{Value: nbt.Compound{
		"LevelName": name, "SpawnX": spawn[0], "SpawnY": spawn[1], "SpawnZ": spawn[2],
		"GameType": int32(1), "commandsAllowed": int8(1), "Generator": int32(2), "FlatWorldLayers": `{"biome_id":1,"block_layers":[]}`,
	}}}, export.DataVersion)
	if err != nil {
		return count, err
	}
	if s.highY > 320 {
		if err := writeHeightPack(dir, &level, s.highY); err != nil {
			return count, err
		}
	}
	if err = world.WriteLevel(level); err != nil {
		return count, err
	}
	return count, world.Commit()
}

// readBucket 依次解压一个片区的记录，Java 与 Bedrock 共用空间重排结果。
func (s *blockSpool) readBucket(ctx context.Context, p bucketPos, b *bucket, visit func(x, y, z, length int32, state uint32) error) error {
	raw := make([]byte, 64*1024)
	compressed := make([]byte, 128*1024)
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
		_, err = io.ReadFull(reader, raw[:part.raw])
		if err != nil {
			return err
		}
		for data := raw[:part.raw]; len(data) > 0; data = data[12:] {
			start, n, state := binary.LittleEndian.Uint32(data), binary.LittleEndian.Uint32(data[4:]), binary.LittleEndian.Uint32(data[8:])
			if err := visit(p.x*128+int32(start&127), int32(start>>14)-64, p.z*128+int32(start>>7&127), int32(n), state); err != nil {
				return err
			}
		}
	}
	return nil
}

// clearTiles 让后一个投影区域的方块覆盖同时移除旧区域的方块实体。
func (s *blockSpool) clearTiles(minimum, size [3]int32) {
	for _, b := range s.buckets {
		kept := b.tiles[:0]
		for _, tile := range b.tiles {
			p := tile.Pos
			if p.X >= minimum[0] && p.X < minimum[0]+size[0] && p.Y >= minimum[1] && p.Y < minimum[1]+size[1] && p.Z >= minimum[2] && p.Z < minimum[2]+size[2] {
				continue
			}
			kept = append(kept, tile)
		}
		b.tiles = kept
	}
}
