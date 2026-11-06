package structure

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	nbt "neomega-nbt/presets/v1"
	"os"
)

// readNBT 直接从文件解压到调用方的接收器，结束时校验 gzip 尾部和多余数据。
func readNBT(ctx context.Context, path string, target any, encoding nbt.Encoding, report func(Progress)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	counted := &progressReader{reader: contextReader{ctx, file}, total: stat.Size(), report: report}
	report(Progress{Stage: "reading", Total: stat.Size()})
	reader := bufio.NewReaderSize(counted, 64*1024)
	magic, err := reader.Peek(2)
	if err != nil {
		return err
	}
	if magic[0] == 0x1f && magic[1] == 0x8b {
		zipped, err := gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer zipped.Close()
		reader = bufio.NewReaderSize(zipped, 128*1024)
	}
	options := nbt.OptDecodeFromNBT{Encoding: encoding}
	if err := nbt.DecodeFromNBT(reader, target, options); err != nil {
		return err
	}
	if _, err = reader.ReadByte(); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("structure: trailing NBT data")
	}
	report(Progress{Stage: "arranging"})
	return ctx.Err()
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

// nbtInt32 读取 NBT 的 byte、short、int，以及放得进 int32 的 long。
// 结构文件里尺寸有时是 short，有时是 int，调用处不应各自再判断类型。
func nbtInt32(value any) (int32, bool) {
	switch typed := value.(type) {
	case int8:
		return int32(typed), true
	case int16:
		return int32(typed), true
	case int32:
		return typed, true
	case int64:
		if typed != int64(int32(typed)) {
			return 0, false
		}
		return int32(typed), true
	default:
		return 0, false
	}
}

// int3 接收结构坐标的 IntArray 或 Int List，其余 NBT 数组仍保留原始标签类型。
func int3(value any) ([3]int32, bool) {
	switch pos := value.(type) {
	case [3]int32:
		return pos, true
	case []int32:
		if len(pos) == 3 {
			return [3]int32{pos[0], pos[1], pos[2]}, true
		}
	}
	return [3]int32{}, false
}

// progressReader 计量实际读入的源文件字节，不额外读取或缓存数据。
type progressReader struct {
	reader      io.Reader
	done, total int64
	report      func(Progress)
}

func (r *progressReader) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	r.done += int64(n)
	r.report(Progress{Stage: "reading", Completed: r.done, Total: r.total})
	return n, err
}
