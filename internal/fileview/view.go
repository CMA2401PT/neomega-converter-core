// Package fileview 为已经写完的临时文件提供并发只读视图。
// 映射期间不得改写或截断文件；所有读取结束后 Close，原文件句柄仍由调用方持有。
package fileview

import (
	"io"
	"os"
)

type View struct {
	file        io.ReaderAt
	data        []byte
	MappedBytes int64
	release     func() error
}

// Open 在预算足够时映射文件；超出预算或平台未实现映射时，保留按偏移读取。
// 两条路径都不会将整份文件复制进 Go 堆；MappedBytes 非零表示这次确实映射了。
func Open(file *os.File, maxBytes int64) (*View, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	view := &View{file: file}
	if info.Size() == 0 || info.Size() > maxBytes {
		return view, nil
	}
	data, release, err := mapFile(file, int(info.Size()))
	if err != nil {
		return nil, err
	}
	if data != nil {
		view.data = data
		view.MappedBytes = info.Size()
		view.release = release
	}
	return view, nil
}

// ReadRange 借出映射的一段，只能读取，且不可持有到 Close 之后。
// 未映射时把数据读进调用方的 scratch，scratch 至少要有 size 字节，这样每个压缩片段不必各自分配缓冲。
func (v *View) ReadRange(offset int64, size int, scratch []byte) ([]byte, error) {
	if v.data != nil {
		return v.data[offset : offset+int64(size)], nil
	}
	data := scratch[:size]
	_, err := v.file.ReadAt(data, offset)
	return data, err
}

// Close 必须在所有读者退出后调用，只释放映射，不关闭借用的文件。
func (v *View) Close() error {
	if v.release != nil {
		return v.release()
	}
	return nil
}
