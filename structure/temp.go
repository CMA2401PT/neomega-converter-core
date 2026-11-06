package structure

import (
	"bufio"
	"compress/gzip"
	"os"
)

// tempWriter 是压缩临时文件。调用方把记录写进 records，收尾时依次刷出
// records、zipped、packed 到文件；顺序不能换，gzip 的尾部必须写在压缩流之后。
type tempWriter struct {
	path    string
	file    *os.File
	zipped  *gzip.Writer
	packed  *bufio.Writer // gzip 的输出缓冲
	records *bufio.Writer // 调用方的写入缓冲
}

// newTempWriter 在 dir 下建一个压缩临时文件；调用方用完必须 close 或 discard。
func newTempWriter(dir, pattern string) (*tempWriter, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	w := &tempWriter{path: file.Name(), file: file}
	w.packed = bufio.NewWriterSize(file, 64*1024)
	// BestSpeed 是合法级别，gzip 不会在这里返回错误。
	w.zipped, _ = gzip.NewWriterLevel(w.packed, gzip.BestSpeed)
	w.records = bufio.NewWriterSize(w.zipped, 64*1024)
	return w, nil
}

// close 刷出所有缓冲并关闭文件；重复调用只关一次。
func (w *tempWriter) close() error {
	if w.file == nil {
		return nil
	}
	if err := w.records.Flush(); err != nil {
		return err
	}
	if err := w.zipped.Close(); err != nil {
		return err
	}
	if err := w.packed.Flush(); err != nil {
		return err
	}
	return w.discard()
}

// discard 释放没有正常收尾的临时文件，已关闭时什么也不做。
func (w *tempWriter) discard() error {
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
