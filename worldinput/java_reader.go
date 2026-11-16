package worldinput

import (
	"context"
	"errors"
	"io/fs"

	"celler"
	"celler/java"
	"celler/java/region"
	"neomega-converter-core/bytecache"
)

// chunkRequest 把索引扫描送到拥有 World 的协程。
type chunkRequest struct {
	// 列出坐标也需要独占 World，回复只含索引，不解码原始记录。
	ctx   context.Context
	reply chan chunkResult
}

// chunkResult 只传坐标，扫描不解码区块内容。
type chunkResult struct {
	positions []celler.ChunkPos
	err       error
}

// sourceRequest 把取消范围绑定到一次坐标读取。
type sourceRequest struct {
	ctx   context.Context
	pos   celler.ChunkPos
	reply chan sourceResult
}

// JavaRecords 分别保留 Terrain 和 Entities；任一缺失都不掩盖另一份记录。
type JavaRecords [2]*region.Record

// sourceResult 传回共享原始记录，消费者独立解码。
type sourceResult struct {
	record JavaRecords
	err    error
}

// JavaReader 用一个 goroutine 独占 World，使并发消费者只共享不可变记录。
type JavaReader struct {
	requests chan sourceRequest
	chunks   chan chunkRequest
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewJavaReader 接管文件读取，Close 归还 World 的访问权，句柄仍由打开方关闭。
func NewJavaReader(world *java.World, dim java.Dimension) *JavaReader {
	// 调用方交出 World 的独占访问权，Close 返回后才可关闭 World 及其文件容器。
	ctx, cancel := context.WithCancel(context.Background())
	s := &JavaReader{
		requests: make(chan sourceRequest, 8),
		chunks:   make(chan chunkRequest),
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	go s.run(ctx, world, dim)
	return s
}

// run 把文件访问与解码分离，等待者取消后仍等底层 I/O 完成再归还句柄。
func (s *JavaReader) run(ctx context.Context, world *java.World, dim java.Dimension) {
	defer close(s.done)
	cache := bytecache.New[celler.ChunkPos, JavaRecords](32 << 20)
	for {
		select {
		case <-ctx.Done():
			return
		case request := <-s.chunks:
			positions := make([]celler.ChunkPos, 0)
			err := world.Scan(dim, java.Terrain, func(pos celler.ChunkPos) error {
				if err := request.ctx.Err(); err != nil {
					return err
				}
				positions = append(positions, pos)
				return nil
			})
			request.reply <- chunkResult{positions: positions, err: err}
		case request := <-s.requests:
			if request.ctx.Err() != nil {
				continue
			}
			record, ok := cache.Get(request.pos)
			var err error
			if !ok {
				for index, kind := range []java.RegionKind{java.Terrain, java.Entities} {
					value, readErr := world.ReadRaw(dim, kind, request.pos)
					if errors.Is(readErr, fs.ErrNotExist) {
						continue
					}
					if readErr != nil {
						err = readErr
						break
					}
					record[index] = &value
				}
				if err == nil {
					size := 0
					for _, value := range record {
						if value != nil {
							size += len(value.Data)
						}
					}
					cache.Put(request.pos, record, size)
				}
			}
			request.reply <- sourceResult{
				record: record,
				err:    err,
			}
		}
	}
}

// Read 共享地形和实体的原始字节，使每个消费者能独立解码。
func (s *JavaReader) Read(ctx context.Context, pos celler.ChunkPos) (JavaRecords, error) {
	if err := ctx.Err(); err != nil {
		return JavaRecords{}, err
	}
	reply := make(chan sourceResult, 1)
	select {
	case s.requests <- sourceRequest{
		ctx:   ctx,
		pos:   pos,
		reply: reply,
	}:
	case <-ctx.Done():
		return JavaRecords{}, ctx.Err()
	case <-s.done:
		return JavaRecords{}, fs.ErrClosed
	}
	select {
	case result := <-reply:
		return result.record, result.err
	case <-ctx.Done():
		return JavaRecords{}, ctx.Err()
	case <-s.done:
		return JavaRecords{}, fs.ErrClosed
	}
}

// Close 等待文件 goroutine 退出，调用方随后才能关闭 World。
func (s *JavaReader) Close() {
	// 等文件 goroutine 结束后再归还 World，取消请求本身不保证底层 I/O 已完成。
	// 此方法不关闭 World；调用方在退役相关 View 后先 Close，再关闭 World 和文件容器。
	s.cancel()
	<-s.done
}

// Chunks 将索引扫描交给文件 goroutine，避免并发改动 region 缓存。
func (s *JavaReader) Chunks(ctx context.Context) ([]celler.ChunkPos, error) {
	// 索引扫描也交给独占 World 的 goroutine，避免与原始记录读取争用 region 缓存。
	reply := make(chan chunkResult, 1)
	select {
	case s.chunks <- chunkRequest{ctx: ctx, reply: reply}:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, fs.ErrClosed
	}
	select {
	case result := <-reply:
		return result.positions, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, fs.ErrClosed
	}
}
