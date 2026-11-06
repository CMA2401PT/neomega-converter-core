package structure

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// convertMCWorld 把原生世界包展开成目录：世界文件原样拷贝，不翻译数据库。
func convertMCWorld(j *job) error {
	archive, err := zip.OpenReader(j.input)
	if err != nil {
		return err
	}
	defer archive.Close()
	prefix, err := worldPrefix(archive.File)
	if err != nil {
		return err
	}
	j.report(Progress{Stage: "extracting", Total: int64(len(archive.File))})
	for index, file := range archive.File {
		j.report(Progress{Stage: "extracting", Completed: int64(index), Total: int64(len(archive.File))})
		if err := j.ctx.Err(); err != nil {
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
		target := filepath.Join(j.output, filepath.FromSlash(relative))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := copyEntry(j.ctx, file, target); err != nil {
			return err
		}
	}
	return nil
}

// worldPrefix 找到压缩包里世界的根目录：level.dat 所在的那一层，必须唯一。
func worldPrefix(files []*zip.File) (string, error) {
	prefix := ""
	found := false
	for _, file := range files {
		if path.Base(file.Name) != "level.dat" {
			continue
		}
		candidate := strings.TrimSuffix(file.Name, "level.dat")
		if found && candidate != prefix {
			return "", fmt.Errorf("mcworld: multiple worlds")
		}
		prefix, found = candidate, true
	}
	if !found {
		return "", fmt.Errorf("mcworld: missing level.dat")
	}
	return prefix, nil
}

// copyEntry 把一个 zip 条目写到 target；复制按块读，取消不必等整个条目读完。
func copyEntry(ctx context.Context, file *zip.File, target string) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dest, contextReader{ctx: ctx, reader: source})
	return errors.Join(copyErr, dest.Close())
}
