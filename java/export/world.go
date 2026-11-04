package export

import (
	"fmt"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"neomega-converter-core"
	converterjava "neomega-converter-core/java"
)

type ConvertOptions struct {
	// 空列表转换主世界。维度编号沿用 Bedrock 的 0/1/2。
	Dimensions []int32
	Strict     bool
}

type FailedChunk struct {
	Dimension int32
	Pos       celler.ChunkPos
	Err       error
}

type Report struct {
	Chunks       map[int32]uint64
	FailedChunks []FailedChunk
	Diagnostics  map[converter.DiagnosticKind]uint64
}

var dimensions = map[int32]cellerjava.Dimension{
	0: cellerjava.Overworld,
	1: cellerjava.Nether,
	2: cellerjava.End,
}

// ConvertWorld 逐根读取 Java 柱子，转换后写进目标 Bedrock 世界。
func ConvertWorld(source *cellerjava.World, target *cellerbedrock.World, options ConvertOptions) (Report, error) {
	report := Report{
		Chunks:      make(map[int32]uint64),
		Diagnostics: make(map[converter.DiagnosticKind]uint64),
	}
	ids := options.Dimensions
	if len(ids) == 0 {
		ids = []int32{0}
	}
	for _, id := range ids {
		dim, ok := dimensions[id]
		if !ok {
			return report, fmt.Errorf("unsupported Java dimension %d", id)
		}
		report.Chunks[id] = 0
		err := source.Scan(dim, cellerjava.Terrain, func(pos celler.ChunkPos) error {
			chunk, err := source.ReadChunk(dim, pos)
			if err != nil {
				return report.failed(id, pos, err, options.Strict)
			}
			converted, err := converterjava.ConvertChunk(pos, id, chunk)
			if err != nil {
				return report.failed(id, pos, err, options.Strict)
			}
			if err := WriteChunk(target, converted); err != nil {
				return err
			}
			for _, diagnostic := range converted.Diagnostics {
				count := diagnostic.Count
				if count == 0 {
					count = 1
				}
				report.Diagnostics[diagnostic.Kind] += count
			}
			report.Chunks[id]++
			return nil
		})
		if err != nil {
			return report, err
		}
	}
	return report, nil
}

func (r *Report) failed(dimension int32, pos celler.ChunkPos, err error, strict bool) error {
	r.FailedChunks = append(r.FailedChunks, FailedChunk{Dimension: dimension, Pos: pos, Err: err})
	if strict {
		return err
	}
	return nil
}
