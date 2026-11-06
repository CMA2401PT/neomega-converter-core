// Package pipeline 把串行读取、并行计算和有序写出连接起来，限制未交付任务数量。
package pipeline

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

// Workers 按估算工作内存限制并发。至少执行一个任务，预算不是进程 RSS 硬上限。
// reserved 包含映射和写入批次；taskBytes 包含一个任务的输入、工作区和输出。
func Workers(memoryMiB, requested int, reserved, taskBytes int64) (int, error) {
	if memoryMiB == 0 {
		memoryMiB = 512
	}
	if memoryMiB < 64 || requested < 0 {
		return 0, fmt.Errorf("转换内存预算至少 64 MiB，workers 不能为负数")
	}
	if requested == 0 {
		requested = min(runtime.GOMAXPROCS(0), 8)
	}
	return min(requested, max(1, int((int64(memoryMiB)*1024*1024-reserved)/taskBytes))), nil
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
