// Package worldinput 为浏览和转换应用准备原生世界，共用路径、维度和句柄生命周期。
package worldinput

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"celler"
	"celler/bedrock"
	"celler/java"
	"neomega-converter-core/structure"
	nbt "neomega-nbt/presets/v1"
	packfs "neomega-packfs"
)

// Options 将结构准备与原生读取放在同一次打开中，维度和位置不会在转换后丢失。
type Options struct {
	Path          string
	Dimension     string
	Position      *[3]float64
	Structure     structure.Options
	StructureRoot string
}

// Info 同时保留用户选择的入口和实际世界路径，便于解释结构输出的复用。
type Info struct {
	Name              string     `json:"name"`
	Path              string     `json:"path"`
	WorldPath         string     `json:"worldPath"`
	Kind              string     `json:"kind"`
	Edition           string     `json:"edition"`
	Dimension         string     `json:"dimension"`
	Dimensions        []string   `json:"dimensions"`
	Position          [3]float64 `json:"position"`
	ReusedStructure   bool       `json:"reusedStructure,omitempty"`
	ConversionSeconds *float64   `json:"conversionSeconds,omitempty"`
}

// Source 拥有读取器与原生句柄，关闭时先归还在途读取再释放文件和容器。
type Source struct {
	Info    Info
	Java    *JavaReader
	Bedrock *BedrockReads
	close   func() error
}

// Open 先准备结构输出，再打开所选维度，失败时释放已经取得的句柄。
func Open(ctx context.Context, options Options) (_ *Source, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input, err := Resolve(options.Path)
	if err != nil {
		return nil, err
	}
	source := &Source{Info: Info{Path: input.Path, Kind: input.Kind}}
	if kind := structure.FileKind(input.Path); kind != "" && kind == input.Kind {
		directory := structure.WorldDirectory(input.Path)
		if options.StructureRoot != "" {
			name := structure.WorldDirectory(filepath.Base(input.Path))
			if !filepath.IsLocal(name) || name == "." {
				return nil, fmt.Errorf("无效的转换目录名：%q", name)
			}
			directory = filepath.Join(options.StructureRoot, name)
		}
		entries, err := os.ReadDir(directory)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		source.Info.ReusedStructure = len(entries) > 0
		if !source.Info.ReusedStructure {
			start := time.Now()
			if _, err := structure.ConvertFile(ctx, input.Path, directory, options.Structure); err != nil {
				return nil, err
			}
			seconds := time.Since(start).Seconds()
			source.Info.ConversionSeconds = &seconds
		}
		input, err = Resolve(directory)
		if err != nil {
			return nil, err
		}
	}
	source.Info.WorldPath = input.Path
	source.Info.Name = filepath.Base(input.Path)
	defer func() {
		if err != nil && source.close != nil {
			source.Close()
		}
	}()
	if input.Kind == "bedrock" {
		dimension := int64(0)
		if options.Dimension != "" {
			dimension, err = strconv.ParseInt(options.Dimension, 10, 32)
			if err != nil || dimension < 0 || dimension > 2 {
				return nil, fmt.Errorf("Bedrock dimension 应为 0、1 或 2")
			}
		}
		world, err := bedrock.Open(input.Path, bedrock.Options{ReadOnly: true})
		if err != nil {
			return nil, err
		}
		source.close = world.Close
		source.Info.Edition = "bedrock"
		source.Info.Dimension = strconv.FormatInt(dimension, 10)
		source.Info.Dimensions = []string{"0", "1", "2"}
		source.Info.Position, err = bedrockPosition(world, int32(dimension))
		if err != nil {
			return nil, err
		}
		reads := &BedrockReads{
			ReadChunk: func(ctx context.Context, pos celler.ChunkPos) (*bedrock.Chunk, error) {
				return world.ReadChunk(int32(dimension), pos)
			},
			ReadChunkActors: func(ctx context.Context, pos celler.ChunkPos) ([]nbt.Compound, error) {
				documents, err := world.ReadChunkActors(int32(dimension), pos)
				if err != nil {
					return nil, err
				}
				values := make([]nbt.Compound, len(documents))
				for i, document := range documents {
					values[i] = document.Value
				}
				return values, nil
			},
			ScanChunks: func(ctx context.Context) ([]celler.ChunkPos, error) {
				return BedrockChunks(ctx, world, int32(dimension))
			},
		}
		source.Bedrock = reads
		source.close = func() error { reads.Close(); return world.Close() }
	} else {
		var world *java.World
		var container *packfs.PackFS
		if input.Kind == "pack" {
			container, err = packfs.OpenReadOnly(input.Path)
			if err != nil {
				return nil, err
			}
			source.close = container.Close
			world, err = java.Open(container, java.Options{ReadOnly: true, MaxOpenRegions: 16})
		} else {
			world, err = java.OpenDirectory(input.Path, java.Options{ReadOnly: true, MaxOpenRegions: 16})
		}
		if err != nil {
			return nil, err
		}
		source.close = func() error {
			worldErr := world.Close()
			if container != nil {
				return errors.Join(worldErr, container.Close())
			}
			return worldErr
		}
		source.Info.Edition = "java"
		dim, dimensions, err := javaDimensions(world.Files, options.Dimension)
		if err != nil {
			return nil, err
		}
		source.Info.Dimension = string(dim)
		source.Info.Dimensions = dimensions
		source.Info.Position, err = javaPosition(world, dim)
		if err != nil {
			return nil, err
		}
		source.Java = NewJavaReader(world, dim)
		closeWorld := source.close
		source.close = func() error { source.Java.Close(); return closeWorld() }
	}
	if options.Position != nil {
		source.Info.Position = *options.Position
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return source, nil
}

// Close 等待读取器退出，保证转换缓存的迟到回调不会触碰已关闭的数据库。
func (s *Source) Close() error {
	return s.close()
}
