package bedrockout

import cellerbedrock "celler/bedrock"

// BatchBytes 是一个写入批次保留的编码记录上限。提交时数据库 batch 还会复制一份，
// 所以调用方按两倍从内存预算里预留。
const BatchBytes = 8 * 1024 * 1024

// Batch 把编码好的柱子攒成有界批次原子提交，每个批次里地形和实体一起替换。
// 只能由一个协程使用。
type Batch struct {
	world    *cellerbedrock.World
	options  cellerbedrock.WriteChunksOptions
	pending  []*cellerbedrock.EncodedChunk
	onCommit []func()
	size     int
}

// NewBatch 创建写入 world 的批次。newWorld 表示目标坐标都还没写过，可以跳过旧记录清理。
func NewBatch(world *cellerbedrock.World, newWorld bool) *Batch {
	return &Batch{world: world, options: cellerbedrock.WriteChunksOptions{NewWorld: newWorld}}
}

// Add 加入一根柱子。committed 在这根柱子真正提交后调用，可以为 nil；
// 报告据此只计入已经落盘的柱子。超大单柱单独提交。
func (b *Batch) Add(chunk *cellerbedrock.EncodedChunk, committed func()) error {
	if b.size+chunk.Size() > BatchBytes {
		if err := b.Flush(); err != nil {
			return err
		}
	}
	b.pending = append(b.pending, chunk)
	b.onCommit = append(b.onCommit, committed)
	b.size += chunk.Size()
	if b.size >= BatchBytes {
		return b.Flush()
	}
	return nil
}

// Flush 提交所有待写的柱子。提交失败的批次直接丢弃，之后再调用 Flush 不会重试它。
func (b *Batch) Flush() error {
	if len(b.pending) == 0 {
		return nil
	}
	err := b.world.WriteChunks(b.pending, b.options)
	if err == nil {
		for _, committed := range b.onCommit {
			if committed != nil {
				committed()
			}
		}
	}
	clear(b.pending)
	clear(b.onCommit)
	b.pending, b.onCommit, b.size = b.pending[:0], b.onCommit[:0], 0
	return err
}
