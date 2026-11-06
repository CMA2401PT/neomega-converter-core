package structure

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// extractMCWorld 将原生世界包展开为目录，保留数据库和世界的其他文件。
func extractMCWorld(ctx context.Context, input, output string, report func(Progress)) error {
	archive, err := zip.OpenReader(input)
	if err != nil {
		return err
	}
	defer archive.Close()
	prefix := ""
	found := false
	for _, file := range archive.File {
		if path.Base(file.Name) == "level.dat" {
			candidate := strings.TrimSuffix(file.Name, "level.dat")
			if found && candidate != prefix {
				return fmt.Errorf("mcworld: multiple worlds")
			}
			prefix = candidate
			found = true
		}
	}
	if !found {
		return fmt.Errorf("mcworld: missing level.dat")
	}
	report(Progress{Stage: "extracting", Total: int64(len(archive.File))})
	for index, file := range archive.File {
		report(Progress{Stage: "extracting", Completed: int64(index), Total: int64(len(archive.File))})
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, ok := strings.CutPrefix(file.Name, prefix)
		if !ok || relative == "" {
			continue
		}
		// zip 条目来自输入文件，必须限制在输出目录内。
		if strings.Contains(relative, "\\") || !filepath.IsLocal(filepath.FromSlash(relative)) {
			return fmt.Errorf("mcworld: invalid path %q", file.Name)
		}
		target := filepath.Join(output, filepath.FromSlash(relative))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		err := func() error {
			source, err := file.Open()
			if err != nil {
				return err
			}
			defer source.Close()
			dest, err := os.Create(target)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(dest, contextReader{ctx, source})
			closeErr := dest.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
