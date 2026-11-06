// Package pipeline 把串行读取、并行计算和有序写出连接起来，限制未交付任务数量。
package pipeline

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

// DefaultMemoryMiB 是未指定内存预算时的默认值。展示或校验默认值的调用方读这里，
// 不要另写一份，否则两处默认值会悄悄分叉。
const DefaultMemoryMiB = 512

// Limit 是一次并发计算的预算。单位分开写在字段名里，调用方按字段名填，
// 不必再记参数顺序。
type Limit struct {
	// MemoryMiB 是总预算；零值用 DefaultMemoryMiB。
	MemoryMiB int
	// Requested 是调用方要求的任务数；零值按 CPU 自动选择，最多 8 个。
	Requested int
	// Reserved 是映射与写出批次占用的字节，先从总预算里扣掉。
	Reserved int64
	// TaskBytes 是单个任务的输入、工作区与输出，必须大于零。
	TaskBytes int64
}

// Workers 按预算限制并发数，至少执行一个任务。预算不是进程 RSS 硬上限。
func Workers(limit Limit) (int, error) {
	memoryMiB := limit.MemoryMiB
	if memoryMiB == 0 {
		memoryMiB = DefaultMemoryMiB
	}
	if memoryMiB < 64 || limit.Requested < 0 {
		return 0, fmt.Errorf("转换内存预算至少 64 MiB，workers 不能为负数")
	}
	requested := limit.Requested
	if requested == 0 {
		requested = min(runtime.GOMAXPROCS(0), 8)
	}
	available := int64(memoryMiB)*1024*1024 - limit.Reserved
	return min(requested, max(1, int(available/limit.TaskBytes))), nil
}

type result[T any] struct {
	value T
	err   error
}
type task[I, O any] struct {
	input I
	done  chan result[O]
}

// Run 的 produce 独占输入句柄，work 独占每个输入，consume 在调用协程按输入顺序执行。
// 最多有 workers 个未交付任务，外加生产者正在读取的一个输入；慢任务不会导致结果无限积压。
// workers 至少为 1，本包的 Workers 保证这一点。
// 返回前停止并等待全部任务，调用者随后可以安全关闭源文件或解除映射。
func Run[I, O any](
	ctx context.Context,
	workers int,
	produce func(context.Context, func(I) error) error,
	work func(context.Context, I) (O, error),
	consume func(O) error,
) error {
	ctx, cancel := context.WithCancel(ctx)
	var group sync.WaitGroup
	defer func() { cancel(); group.Wait() }()
	jobs := make(chan task[I, O])
	order := make(chan task[I, O], workers)
	// slots 是未交付任务的名额：生产者取到名额才入队，消费完一个才放回。
	slots := make(chan struct{}, workers)
	produced := make(chan error, 1)
	group.Go(func() {
		defer close(jobs)
		defer close(order)
		produced <- produce(ctx, func(input I) error {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			job := task[I, O]{input: input, done: make(chan result[O], 1)}
			select {
			case order <- job:
			case <-ctx.Done():
				return ctx.Err()
			}
			select {
			case jobs <- job:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	})
	for range workers {
		group.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					value, err := work(ctx, job.input)
					job.done <- result[O]{value, err}
				}
			}
		})
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case job, ok := <-order:
			if !ok {
				if err := ctx.Err(); err != nil {
					return err
				}
				return <-produced
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case output := <-job.done:
				if output.err != nil {
					return output.err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := consume(output.value); err != nil {
					return err
				}
				<-slots
			}
		}
	}
}
