//go:build !darwin && !linux && !windows

package fileview

import "os"

func mapFile(*os.File, int) ([]byte, func() error, error) { return nil, nil, nil }
