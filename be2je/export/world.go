package export

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"neomega-converter-core"
	"neomega-converter-core/be2je"
	"neomega-converter-core/internal/pipeline"
	"neomega-converter-core/javaout"
	nbt "neomega-nbt/presets/v1"
)

type ConvertOptions struct {
	// Context 控制读取、计算与写出；nil 使用 Background。返回前等待所有任务退出。
	Context context.Context
	// MemoryMiB 默认 512、至少 64；Workers 为零时自动选择，预算按每任务 64 MiB 估算。
	MemoryMiB int
	Workers   int
	// 空维度列表选择主世界；Strict 只控制转换失败的柱子。
	Dimensions []int32
	Strict     bool
	// IncludeUnfinished 也转换尚未生成完成的柱子；默认跳过，和 mcconv 一致。
	IncludeUnfinished bool
}

type Report struct {
	// 计数以实际写出为准；诊断包括写出失败的柱子，它们同样说明映射缺了什么。
	Chunks          map[int32]uint64
	FailedChunks    []converter.FailedChunk
	Diagnostics     converter.DiagnosticSummary
	DroppedSections map[int32]uint64
	Skipped         uint64
	BlockEntities   uint64
	Entities        uint64
}

// finalizedState 读取 0x36 记录的生成状态。缺失按已完成处理，和 mcconv 一致。
func finalizedState(chunk *cellerbedrock.Chunk) int32 {
	data, ok := chunk.Records[string([]byte{cellerbedrock.TagFinalizedState})]
	if !ok {
		return 2
	}
	switch len(data) {
	case 1:
		return int32(int8(data[0]))
	case 4:
		return int32(binary.LittleEndian.Uint32(data))
	default:
		return 2
	}
}

// chunkTaskBytes 是一根柱子在任务内转换、编码和压缩的工作区估算。
const chunkTaskBytes = 64 * 1024 * 1024

func ConvertWorld(source *cellerbedrock.World, target *cellerjava.World, options ConvertOptions) (report Report, err error) {
	// 源扫描与目标写入各有唯一所有者，转换、NBT 编码和压缩在独立任务中执行。
	workers, err := pipeline.Workers(pipeline.Limit{
		MemoryMiB: options.MemoryMiB,
		Requested: options.Workers,
		TaskBytes: chunkTaskBytes,
	})
	if err != nil {
		return report, err
	}
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	defer func() { err = errors.Join(err, target.Flush()) }()
	report = Report{
		Chunks:          make(map[int32]uint64),
		Diagnostics:     converter.NewDiagnosticSummary(),
		DroppedSections: make(map[int32]uint64),
	}
	dims := options.Dimensions
	if len(dims) == 0 {
		dims = []int32{0}
	}
	selected := make(map[int32]cellerjava.Dimension)
	for _, dimension := range dims {
		dim, ok := javaout.Dimension(dimension)
		if !ok {
			return report, fmt.Errorf("unsupported Bedrock dimension %d", dimension)
		}
		selected[dimension] = dim
		report.Chunks[dimension], report.DroppedSections[dimension] = 0, 0
	}
	err = pipeline.Run(ctx, workers,
		func(ctx context.Context, yield func(bedrockInput) error) error {
			return source.ScanChunks(func(raw *cellerbedrock.Chunk) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if _, included := selected[raw.Dimension]; !included {
					return nil
				}
				input := bedrockInput{chunk: raw}
				input.skipped = !options.IncludeUnfinished && finalizedState(raw) < 2
				if !input.skipped {
					documents, err := source.ReadChunkActors(raw.Dimension, raw.Pos)
					if err != nil {
						input.actorErr = fmt.Errorf("actors: %w", err)
					} else {
						for _, document := range documents {
							input.actors = append(input.actors, document.Value)
						}
					}
				}
				return yield(input)
			})
		},
		func(ctx context.Context, input bedrockInput) (javaOutput, error) {
			if input.skipped {
				return javaOutput{skipped: true}, nil
			}
			return encodeBedrockChunk(ctx, input)
		},
		func(output javaOutput) error {
			if output.skipped {
				report.Skipped++
				return nil
			}
			report.Diagnostics.Add(output.diagnostics)
			report.DroppedSections[output.dimension] += output.dropped
			if output.convertErr != nil {
				report.FailedChunks = append(report.FailedChunks, converter.FailedChunk{Dimension: output.dimension, Pos: output.pos, Err: output.convertErr})
				if options.Strict {
					return fmt.Errorf("dimension %d chunk %v: %w", output.dimension, output.pos, output.convertErr)
				}
				return nil
			}
			if err := output.records.Write(target, selected[output.dimension], output.pos); err != nil {
				return err
			}
			report.Entities += uint64(output.records.EntityCount)
			report.BlockEntities += uint64(output.records.BlockEntityCount)
			report.Chunks[output.dimension]++
			return nil
		},
	)
	return report, err
}

// 扫描者交付的记录与实体只属于一个任务，不借用迭代器的可变缓冲。
type bedrockInput struct {
	chunk    *cellerbedrock.Chunk
	actors   []nbt.Compound
	actorErr error
	// skipped 表示这根柱子还没生成完成，任务直接交付空结果、写出侧只计数。
	skipped bool
}

// javaOutput 是一根柱子转换后的结果；转换失败放进 convertErr 而不是作为错误返回，
// 让写出侧按 Strict 决定是否提前结束，并照常统计诊断。
type javaOutput struct {
	pos         celler.ChunkPos
	dimension   int32
	records     javaout.Records
	diagnostics []converter.Diagnostic
	dropped     uint64
	// skipped 由任务在未生成完成的柱子上置位，写出侧只计数、不写出。
	skipped    bool
	convertErr error
}

// encodeBedrockChunk 在任务内完成转换、编码和压缩；只有轻量诊断和压缩记录交给写入者。
func encodeBedrockChunk(ctx context.Context, input bedrockInput) (javaOutput, error) {
	out := javaOutput{pos: input.chunk.Pos, dimension: input.chunk.Dimension}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	chunk, err := be2je.ConvertChunkWithActors(input.chunk, input.actors)
	if err != nil {
		out.convertErr = err
		return out, nil
	}
	if input.actorErr != nil {
		chunk.Diagnostics = append(chunk.Diagnostics, converter.Diagnostic{Kind: converter.NBTError, Err: input.actorErr})
	}
	out.diagnostics = chunk.Diagnostics
	height := javaout.DimensionHeight(chunk.Dimension)
	for y := range chunk.Sections {
		if y < height.Low || y > height.High {
			out.dropped++
		}
	}
	// 编码失败只影响这一根柱子，和转换失败一样交给写出侧按 Strict 处理。
	out.records, out.convertErr = javaout.Encode(chunk, height)
	return out, nil
}
