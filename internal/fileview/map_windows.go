package fileview

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

func mapFile(file *os.File, size int) ([]byte, func() error, error) {
	handle, err := syscall.CreateFileMapping(syscall.Handle(file.Fd()), nil, syscall.PAGE_READONLY, 0, 0, nil)
	if err != nil {
		return nil, nil, err
	}
	address, err := syscall.MapViewOfFile(handle, syscall.FILE_MAP_READ, 0, 0, uintptr(size))
	if err != nil {
		syscall.CloseHandle(handle)
		return nil, nil, err
	}
	// 地址来自 Windows 映射，生命周期由 UnmapViewOfFile 管理，不属于 Go 堆。
	pointer := *(*unsafe.Pointer)(unsafe.Pointer(&address))
	data := unsafe.Slice((*byte)(pointer), size)
	return data, func() error { return errors.Join(syscall.UnmapViewOfFile(address), syscall.CloseHandle(handle)) }, nil
}
