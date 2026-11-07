package adapt

import (
	"context"
	"errors"
	"io/fs"

	"celler"
	cellerbedrock "celler/bedrock"
	"neomega-converter-core"
	"neomega-converter-core/be2je"
	nbt "neomega-nbt/presets/v1"
)

type ChunkSource interface {
	// 消费方只需按坐标取柱子；缺失返回 nil, nil，结果及其引用的数据共享只读。
	Chunk(ctx context.Context, pos celler.ChunkPos) (*converter.JavaChunk, error)
}

type ReadChunk func(context.Context, celler.ChunkPos) (*cellerbedrock.Chunk, error)

// ReadActors 读取与地形分开存储的 digp/actorprefix 实体。
type ReadActors func(context.Context, celler.ChunkPos) ([]nbt.Compound, error)

// 回调绑定一个来源和维度，缺失沿用 celler 的 fs.ErrNotExist。
// 不同坐标可能并发读取，回调负责底层句柄的并发与关闭，并在返回后保持记录只读。

type Options struct {
	// 每层按估算字节独立淘汰最久未使用的结果；零值使用 64 MiB，负值只合并在途请求。
	CacheBytes int64
	// StateFallback is only for previewing states outside the export preset.
	StateFallback be2je.StateFallback
}

type Converted struct {
	// 缓存只保存阶段 1，让多个中心修复时共享相同的原始邻居。
	cache *CachedSource
}

func NewConverted(read ReadChunk, options Options, actors ...ReadActors) *Converted {
	// 来源内容在实例存活期间保持稳定；重新打开或修改来源后创建新实例。
	load := func(
		ctx context.Context,
		pos celler.ChunkPos,
	) (*converter.JavaChunk, error) {
		raw, err := read(ctx, pos)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var extra []nbt.Compound
		var actorErr error
		if len(actors) > 0 {
			extra, actorErr = actors[0](ctx, pos)
		}
		converted, err := be2je.ConvertChunkWithStateFallback(raw, extra, options.StateFallback)
		if err != nil {
			return nil, err
		}
		if actorErr != nil {
			converted.Diagnostics = append(converted.Diagnostics, converter.Diagnostic{Kind: converter.NBTError, Err: actorErr})
		}
		return converted, nil
	}
	return &Converted{cache: NewCachedSource(options, load)}
}

func (c *Converted) Chunk(ctx context.Context, pos celler.ChunkPos) (*converter.JavaChunk, error) {
	// 单个等待者退出不取消其他调用者正在共用的读取和转换。
	return c.cache.Chunk(ctx, pos)
}
