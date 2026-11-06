package fileview

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// 同一文件可由多个任务读取；预算不足时仍走相同的范围读取接口。
func TestConcurrentRangesAndBorrowedFile(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 4096)
	path := filepath.Join(t.TempDir(), "spool")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, budget := range []int64{0, int64(len(data))} {
		view, err := Open(file, budget)
		if err != nil {
			t.Fatal(err)
		}
		if budget == 0 && view.MappedBytes != 0 {
			t.Fatal("超预算映射")
		}
		var group sync.WaitGroup
		for worker := range 8 {
			group.Go(func() {
				scratch := make([]byte, 257)
				for offset := worker; offset+len(scratch) <= len(data); offset += 997 {
					got, err := view.ReadRange(int64(offset), len(scratch), scratch)
					if err != nil || !bytes.Equal(got, data[offset:offset+len(scratch)]) {
						t.Errorf("offset=%d: %v", offset, err)
						return
					}
				}
			})
		}
		group.Wait()
		if err := view.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := file.ReadAt(make([]byte, 1), 0); err != nil {
			t.Fatal("借用的文件被关闭", err)
		}
	}
}

func TestEmptyFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "empty")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	view, err := Open(file, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if view.MappedBytes != 0 {
		t.Fatal(view.MappedBytes)
	}
	if err := view.Close(); err != nil {
		t.Fatal(err)
	}
}
