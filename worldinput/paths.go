package worldinput

import (
	"fmt"
	"neomega-converter-core/structure"
	"os"
	"path/filepath"
	"strings"
)

// Input 保留可复用的来源识别结果，让命令行和文件浏览器使用同一分类。
type Input struct{ Path, Kind string }

// FileKind 只按文件名分类，列目录时不打开每个世界容器。
func FileKind(name string) string {
	if strings.EqualFold(filepath.Base(name), "level.dat") {
		return "level"
	}
	if kind := structure.FileKind(name); kind != "" {
		return kind
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pack", ".packfs":
		return "pack"
	}
	return "unknown"
}

// DirectoryKind 允许缺少 level.dat 的导出世界，Bedrock 的 db 优先于 Java 标志。
func DirectoryKind(name string) string {
	if stat, err := os.Stat(filepath.Join(name, "db")); err == nil && stat.IsDir() {
		return "bedrock"
	}
	for _, child := range []string{"level.dat", "region", "dimensions", "DIM-1", "DIM1"} {
		if _, err := os.Stat(filepath.Join(name, child)); err == nil {
			return "directory"
		}
	}
	return "folder"
}

// Resolve 把 level.dat 和带引号的终端路径归一成实际来源，供所有打开入口共用。
func Resolve(name string) (Input, error) {
	name = strings.Trim(strings.TrimSpace(name), "\"")
	if name == "" {
		return Input{}, fmt.Errorf("请选择存档目录、level.dat、PackFS 或结构文件")
	}
	absolute, err := filepath.Abs(name)
	if err != nil {
		return Input{}, err
	}
	stat, err := os.Stat(absolute)
	if err != nil {
		return Input{}, err
	}
	if strings.EqualFold(filepath.Base(absolute), "level.dat") && !stat.IsDir() {
		absolute = filepath.Dir(absolute)
		stat, err = os.Stat(absolute)
		if err != nil {
			return Input{}, err
		}
	}
	if stat.IsDir() {
		kind := DirectoryKind(absolute)
		if kind == "folder" {
			return Input{}, fmt.Errorf("此目录不是存档：%s", absolute)
		}
		return Input{Path: absolute, Kind: kind}, nil
	}
	if !stat.Mode().IsRegular() || stat.Size() == 0 {
		return Input{}, fmt.Errorf("请选择非空的 PackFS 或结构文件")
	}
	kind := FileKind(absolute)
	if kind == "unknown" {
		kind = "pack"
	}
	return Input{Path: absolute, Kind: kind}, nil
}
