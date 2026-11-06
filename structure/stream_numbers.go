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

// numberStream 在 NBT 接收端合并连续重复值，只把游程写进临时文件。
// 元数据可出现在数组之后，因此读取数组时不能假定尺寸或调色板已经可用。
type numberStream struct {
	dir     string
	temp    *tempWriter
	number  streamNumber
	value   uint64
	run     uint64
	count   int64
	varints bool
	partial uint32
	shift   uint
}

// streamNumber 是数组元素的接收器：每个元素推进一个计数为 1 的游程。
type streamNumber struct{ stream *numberStream }

func (v *streamNumber) GetReciver(rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	return goencode.ValueReciver{TrivialValReciver: func(_ any, n int64) error { return v.stream.push(uint64(n), 1) }}, nil
}

func (s *numberStream) GetReciver(t rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	if t.IsList() {
		return goencode.ValueReciver{ListConstructor: &numberList{numberStream: s}}, nil
	}
	return goencode.ValueReciver{ArrayConstructor: s}, nil
}

func (s *numberStream) start() error {
	temp, err := newTempWriter(s.dir, "numbers-*")
	if err != nil {
		return err
	}
	s.temp = temp
	s.number.stream = s
	return nil
}

func (s *numberStream) GetReusableElementPointer(rawnbt.HighLevelValueType, int) (any, error) {
	if err := s.start(); err != nil {
		return nil, err
	}
	return &s.number, nil
}
func (s *numberStream) OnNextElement() error     { return nil }
func (s *numberStream) HasFlag(rawnbt.Flag) bool { return false }
func (s *numberStream) EmptyArray(t rawnbt.HighLevelValueType) error {
	if err := s.start(); err != nil {
		return err
	}
	return s.finish()
}
func (s *numberStream) OnArrayFinish() error { return s.finish() }

// ConsumeByteArray 直接复用一块读缓冲，绕过每字节的 NBT 事件和反射。
// 返回 true 表示接收器已接管这块数据，上层不必再走元素接收路径。
func (s *numberStream) ConsumeByteArray(length int, read func([]byte) error) (bool, error) {
	if err := s.start(); err != nil {
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
				// 非 varint 数据，或者单字节值：整段相同字节合并成一个游程。
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
			// 调色板索引是 32 位 varint，最多 5 字节；第 5 字节只剩 4 位有效。
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

// push 把 count 个相同值并入当前游程，值变化时先写出上一个游程。
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
	_, err := s.temp.records.Write(data[:n])
	s.run = 0
	return err
}

// finish 写出最后一个游程并完成压缩；之后可以 open 读取。
func (s *numberStream) finish() error {
	if err := s.flush(); err != nil {
		return err
	}
	return s.temp.close()
}

// discard 释放没有正常收尾的临时文件。
func (s *numberStream) discard() error {
	if s.temp == nil {
		return nil
	}
	return s.temp.discard()
}

// numberList 是 numberStream 的 List 形式；元素逐个交给 streamNumber。
type numberList struct{ *numberStream }

func (s *numberList) GetReusableElementPointer(t rawnbt.HighLevelValueType, length int, _ bool) (any, error) {
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
	// 文件里没有这个数组时 temp 为空；报错而不是当成全零数据。
	if s.temp == nil {
		return nil, fmt.Errorf("structure: missing numeric array")
	}
	f, err := os.Open(s.temp.path)
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

// next 换到下一个游程。读完整个文件时返回 io.EOF。
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

// one 解出下一个值，游程用尽时自动换到下一条。
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
