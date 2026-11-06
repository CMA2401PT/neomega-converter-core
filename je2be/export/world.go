package export

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"celler/java/region"
	"neomega-converter-core"
	"neomega-converter-core/bedrockout"
	"neomega-converter-core/internal/pipeline"
	"neomega-converter-core/javaout"
	"neomega-converter-core/je2be"
	nbt "neomega-nbt/presets/v1"
)

type ConvertOptions struct {
	// 空列表转换主世界。维度编号沿用 Bedrock 的 0/1/2。
	Dimensions []int32
	Strict     bool
	// Context 为 nil 时使用 Background；返回前等待所有读取和计算退出。
	Context context.Context
	// MemoryMiB 默认 512、至少 64；Workers 的零值按 CPU 自动选择，仍受预算约束。
	MemoryMiB int
	Workers   int
	// NewWorld 仅用于新建目标，跳过逐柱扫描旧记录；默认保留已有世界的替换语义。
	NewWorld bool
}

type FailedChunk struct {
	Dimension int32
	Pos       celler.ChunkPos
	Err       error
}
type Report struct {
	Chunks       map[int32]uint64
	Entities     uint64
	FailedChunks []FailedChunk
	Diagnostics  map[converter.DiagnosticKind]uint64
}

// chunkTaskBytes 是一根柱子在任务内解码、转换和编码的工作区估算。
const chunkTaskBytes = 64 * 1024 * 1024

// ConvertWorld 串行读取独占的压缩记录，并行解码、转换和编码，再按源顺序批量写入。
// 报告仅计入已提交的区块；Strict 失败前的结果仍会提交，取消时丢弃尚未提交的批次。
func ConvertWorld(source *cellerjava.World, target *cellerbedrock.World, options ConvertOptions) (Report, error) {
	report := Report{Chunks: make(map[int32]uint64), Diagnostics: make(map[converter.DiagnosticKind]uint64)}
	workers, err := pipeline.Workers(pipeline.Limit{
		MemoryMiB: options.MemoryMiB,
		Requested: options.Workers,
		Reserved:  2 * bedrockout.BatchBytes,
		TaskBytes: chunkTaskBytes,
	})
	if err != nil {
		return report, err
	}
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ids := options.Dimensions
	if len(ids) == 0 {
		ids = []int32{0}
	}
	for _, id := range ids {
		dim, ok := javaout.Dimension(id)
		if !ok {
			return report, fmt.Errorf("unsupported Java dimension %d", id)
		}
		report.Chunks[id] = 0
		batch := bedrockout.NewBatch(target, options.NewWorld)
		runErr := pipeline.Run(ctx, workers,
			func(ctx context.Context, yield func(javaInput) error) error {
				return source.Scan(dim, cellerjava.Terrain, func(pos celler.ChunkPos) error {
					if err := ctx.Err(); err != nil {
						return err
					}
					input := javaInput{pos: pos}
					input.terrain, input.readErr = source.ReadRaw(dim, cellerjava.Terrain, pos)
					if input.readErr == nil {
						record, err := source.ReadRaw(dim, cellerjava.Entities, pos)
						if err == nil {
							input.actors = &record
						} else if !errors.Is(err, fs.ErrNotExist) {
							input.actorErr = err
						}
					}
					return yield(input)
				})
			},
			func(ctx context.Context, input javaInput) (bedrockOutput, error) {
				return encodeJavaChunk(ctx, input, id)
			},
			func(output bedrockOutput) error {
				if output.convertErr != nil {
					return report.failed(id, output.pos, output.convertErr, options.Strict)
				}
				// 报告只计入已经提交的柱子。
				return batch.Add(output.encoded, func() {
					for _, diagnostic := range output.diagnostics {
						report.Diagnostics[diagnostic.Kind] += max(diagnostic.Count, 1)
					}
					report.Entities += output.entities
					report.Chunks[id]++
				})
			},
		)
		// 严格模式停下时，失败柱子之前的结果仍然提交；取消时丢弃未提交的批次。
		if ctx.Err() == nil {
			runErr = errors.Join(runErr, batch.Flush())
		}
		if runErr != nil {
			return report, runErr
		}
	}
	return report, ctx.Err()
}

type javaInput struct {
	pos               celler.ChunkPos
	terrain           region.Record
	actors            *region.Record
	readErr, actorErr error
}

// bedrockOutput 是一根柱子转换后的编码结果；读记录或转换失败放进 convertErr 而不是
// 作为错误返回，让写出侧按 Strict 决定是否提前结束，并照常统计诊断。
type bedrockOutput struct {
	pos         celler.ChunkPos
	encoded     *cellerbedrock.EncodedChunk
	diagnostics []converter.Diagnostic
	entities    uint64
	convertErr  error
}

// encodeJavaChunk 独占源记录，在任务内展开 NBT 和方块，交付后只保留编码批次与诊断。
func encodeJavaChunk(ctx context.Context, input javaInput, dimension int32) (bedrockOutput, error) {
	out := bedrockOutput{pos: input.pos, convertErr: input.readErr}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if input.readErr != nil {
		return out, nil
	}
	data, err := input.terrain.Decode()
	if err != nil {
		out.convertErr = err
		return out, nil
	}
	chunk, err := cellerjava.DecodeChunk(data)
	if err != nil {
		out.convertErr = err
		return out, nil
	}
	pos, err := chunk.Position()
	if err != nil {
		out.convertErr = err
		return out, nil
	}
	if pos != input.pos {
		out.convertErr = fmt.Errorf("chunk position %v does not match slot %v", pos, input.pos)
		return out, nil
	}
	var actors []nbt.Compound
	actorErr := input.actorErr
	if input.actors != nil {
		data, err := input.actors.Decode()
		if err == nil {
			actors, err = cellerjava.DecodeEntities(data)
		}
		actorErr = err
	}
	converted, err := je2be.ConvertChunkWithActors(pos, dimension, chunk, actors)
	if err != nil {
		out.convertErr = err
		return out, nil
	}
	if actorErr != nil {
		converted.Diagnostics = append(converted.Diagnostics, converter.Diagnostic{Kind: converter.NBTError, Err: actorErr})
	}
	out.diagnostics = converted.Diagnostics
	out.entities = uint64(len(converted.Actors))
	// 编码失败只影响这一根柱子，和转换失败一样交给写出侧按 Strict 处理。
	out.encoded, out.convertErr = bedrockout.EncodeChunk(converted)
	return out, nil
}

func (r *Report) failed(dimension int32, pos celler.ChunkPos, err error, strict bool) error {
	r.FailedChunks = append(r.FailedChunks, FailedChunk{Dimension: dimension, Pos: pos, Err: err})
	if strict {
		return err
	}
	return nil
}
