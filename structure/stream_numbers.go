package structure

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	rawnbt "neomega-nbt"
	goencode "neomega-nbt/encoder-go-object"
)

// numberStream 在 NBT 接收端合并连续重复值，只把游程写入临时文件。
// 元数据可出现在数组之后，因此不在读取数组时假定尺寸或调色板已经可用。
type numberStream struct {
	dir              string
	path             string
	file             *os.File
	zipped           *gzip.Writer
	compressedWriter *bufio.Writer
	writer           *bufio.Writer
	number           streamNumber
	value            uint64
	run              uint64
	count            int64
	declared         int
	varints          bool
	partial          uint32
	shift            uint
}

type streamNumber struct{ stream *numberStream }

func (v *streamNumber) GetReciver(t rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	return goencode.ValueReciver{TrivialValReciver: func(_ any, n int64) error { return v.stream.push(uint64(n), 1) }}, nil
}

func (s *numberStream) GetReciver(t rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	if t.IsList() {
		return goencode.ValueReciver{ListConstructor: &numberList{numberStream: s}}, nil
	}
	return goencode.ValueReciver{ArrayConstructor: s}, nil
}

func (s *numberStream) start(length int) error {
	f, err := os.CreateTemp(s.dir, "numbers-*")
	if err != nil {
		return err
	}
	s.file, s.path, s.declared = f, f.Name(), length
	s.compressedWriter = bufio.NewWriterSize(f, 64*1024)
	s.zipped, _ = gzip.NewWriterLevel(s.compressedWriter, gzip.BestSpeed)
	s.writer = bufio.NewWriterSize(s.zipped, 64*1024)
	s.number.stream = s
	return nil
}
func (s *numberStream) GetReusableElementPointer(t rawnbt.HighLevelValueType, length int) (any, error) {
	if err := s.start(length); err != nil {
		return nil, err
	}
	return &s.number, nil
}
func (s *numberStream) OnNextElement() error     { return nil }
func (s *numberStream) HasFlag(rawnbt.Flag) bool { return false }
func (s *numberStream) EmptyArray(t rawnbt.HighLevelValueType) error {
	if err := s.start(0); err != nil {
		return err
	}
	return s.finish()
}
func (s *numberStream) OnArrayFinish() error { return s.finish() }

// ConsumeByteArray 直接复用一块读缓冲，绕过每字节的 NBT 事件和反射。
func (s *numberStream) ConsumeByteArray(length int, read func([]byte) error) (bool, error) {
	if err := s.start(length); err != nil {
		return true, err
	}
	var buffer [64 * 1024]byte
	for remaining := length; remaining > 0; {
		batch := buffer[:min(remaining, len(buffer))]
		if err := read(batch); err != nil {
			return true, err
		}
		for index := 0; index < len(batch); {
			b := batch[index]
			if !s.varints || (s.shift == 0 && b < 128) {
				end := index + 1
				for end < len(batch) && batch[end] == b {
					end++
				}
				if err := s.push(uint64(b), uint64(end-index)); err != nil {
					return true, err
				}
				index = end
				continue
			}
			index++
			if s.shift == 28 && b > 7 {
				return true, fmt.Errorf("schem: invalid palette varint")
			}
			s.partial |= uint32(b&127) << s.shift
			if b&128 != 0 {
				s.shift += 7
				continue
			}
			if err := s.push(uint64(s.partial), 1); err != nil {
				return true, err
			}
			s.partial, s.shift = 0, 0
		}

		remaining -= len(batch)
	}
	if s.shift != 0 {
		return true, fmt.Errorf("schem: truncated palette varint")
	}
	return true, s.finish()
}
func (s *numberStream) push(value, count uint64) error {
	s.count += int64(count)
	if s.run != 0 && s.value != value {
		if err := s.flush(); err != nil {
			return err
		}
	}
	s.value = value
	s.run += count
	return nil
}
func (s *numberStream) flush() error {
	if s.run == 0 {
		return nil
	}
	var data [20]byte
	n := binary.PutUvarint(data[:], s.value)
	n += binary.PutUvarint(data[n:], s.run)
	_, err := s.writer.Write(data[:n])
	s.run = 0
	return err
}
func (s *numberStream) finish() error {
	if err := s.flush(); err != nil {
		return err
	}
	if err := s.writer.Flush(); err != nil {
		return err
	}
	if err := s.zipped.Close(); err != nil {
		return err
	}
	if err := s.compressedWriter.Flush(); err != nil {
		return err
	}
	return s.file.Close()
}

type numberList struct{ *numberStream }

func (s *numberList) GetReusableElementPointer(t rawnbt.HighLevelValueType, length int, erased bool) (any, error) {
	return s.numberStream.GetReusableElementPointer(t, length)
}
func (s *numberList) NonTypedEmptyList() error                         { return s.EmptyArray(rawnbt.TrivialInt) }
func (s *numberList) TypedEmptyList(t rawnbt.HighLevelValueType) error { return s.EmptyArray(t) }
func (s *numberList) OnListFinish() error                              { return s.finish() }

// runReader 每次只解出一个游程，消费方可以整段跳过空气。
type runReader struct {
	file             *os.File
	zipped           *gzip.Reader
	reader           *bufio.Reader
	value, remaining uint64
}

func (s *numberStream) open() (*runReader, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	zipped, err := gzip.NewReader(bufio.NewReaderSize(f, 1024*1024))
	if err != nil {
		f.Close()
		return nil, err
	}
	return &runReader{file: f, zipped: zipped, reader: bufio.NewReaderSize(zipped, 64*1024)}, nil
}
func (r *runReader) close() { r.zipped.Close(); r.file.Close() }
func (r *runReader) next() error {
	value, err := binary.ReadUvarint(r.reader)
	if err != nil {
		return err
	}
	count, err := binary.ReadUvarint(r.reader)
	if err != nil {
		return err
	}
	r.value, r.remaining = value, count
	return nil
}
func (r *runReader) one() (uint64, error) {
	if r.remaining == 0 {
		if err := r.next(); err != nil {
			return 0, err
		}
	}
	r.remaining--
	return r.value, nil
}

// eachRun 不展开重复值，让空间重排以游程为单位工作。
func (s *numberStream) eachRun(visit func(value, count uint64) error) error {
	r, err := s.open()
	if err != nil {
		return err
	}
	defer r.close()
	for {
		if err := r.next(); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if err := visit(r.value, r.remaining); err != nil {
			return err
		}
	}
}
