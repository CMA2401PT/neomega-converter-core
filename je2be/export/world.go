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
	"neomega-converter-core/internal/pipeline"
	converterjava "neomega-converter-core/je2be"
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

var dimensions = map[int32]cellerjava.Dimension{0: cellerjava.Overworld, 1: cellerjava.Nether, 2: cellerjava.End}

// chunkTaskBytes 是一根柱子在任务内解码、转换和编码的工作区估算。
const chunkTaskBytes = 64 * 1024 * 1024

// ConvertWorld 串行读取独占的压缩记录，并行解码、转换和编码，再按源顺序批量写入。
// 报告仅计入已提交的区块；Strict 失败前的结果仍会提交，取消时丢弃尚未提交的批次。
func ConvertWorld(source *cellerjava.World, target *cellerbedrock.World, options ConvertOptions) (Report, error) {
	report := Report{Chunks: make(map[int32]uint64), Diagnostics: make(map[converter.DiagnosticKind]uint64)}
	// 批次保留 8 MiB 编码记录，提交时数据库 batch 还要复制一份。
	const batchBytes = 8 * 1024 * 1024
	workers, err := pipeline.Workers(pipeline.Limit{
		MemoryMiB: options.MemoryMiB,
		Requested: options.Workers,
		Reserved:  2 * batchBytes,
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
		dim, ok := dimensions[id]
		if !ok {
			return report, fmt.Errorf("unsupported Java dimension %d", id)
		}
		report.Chunks[id] = 0
		var pending []bedrockOutput
		size := 0
		var writeErr error
		flush := func() error {
			if len(pending) == 0 {
				return nil
			}
			chunks := make([]*cellerbedrock.EncodedChunk, len(pending))
			for i, output := range pending {
				chunks[i] = output.encoded
			}
			writeErr = target.WriteChunks(chunks, cellerbedrock.WriteChunksOptions{NewWorld: options.NewWorld})
			if writeErr != nil {
				return writeErr
			}
			for _, output := range pending {
				for _, diagnostic := range output.diagnostics {
					count := diagnostic.Count
					if count == 0 {
						count = 1
					}
					report.Diagnostics[diagnostic.Kind] += count
				}
				report.Entities += output.entities
				report.Chunks[id]++
			}
			clear(pending)
			pending = pending[:0]
			size = 0
			return nil
		}
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
				// 超大单柱单独提交；普通批次在加入下一柱前检查大小。
				if size+output.encoded.Size() > batchBytes {
					if err := flush(); err != nil {
						return err
					}
				}
				pending = append(pending, output)
				size += output.encoded.Size()
				if size >= batchBytes {
					return flush()
				}
				return nil
			},
		)
		if writeErr == nil && ctx.Err() == nil {
			runErr = errors.Join(runErr, flush())
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
	converted, err := converterjava.ConvertChunkWithActors(pos, dimension, chunk, actors)
	if err != nil {
		out.convertErr = err
		return out, nil
	}
	if actorErr != nil {
		converted.Diagnostics = append(converted.Diagnostics, converter.Diagnostic{Kind: converter.NBTError, Err: actorErr})
	}
	out.diagnostics = converted.Diagnostics
	out.entities = uint64(len(converted.Actors))
	out.encoded, err = EncodeChunk(converted)
	return out, err
}

func (r *Report) failed(dimension int32, pos celler.ChunkPos, err error, strict bool) error {
	r.FailedChunks = append(r.FailedChunks, FailedChunk{Dimension: dimension, Pos: pos, Err: err})
	if strict {
		return err
	}
	return nil
}
