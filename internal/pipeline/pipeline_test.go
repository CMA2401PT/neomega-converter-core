package pipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// 故意让第一个任务最后完成，确认提交顺序稳定，且慢任务不会放大在途数量。
func TestOrderedAndBounded(t *testing.T) {
	release := make(chan struct{})
	var outstanding atomic.Int32
	next := 0
	err := Run(context.Background(), 4,
		func(ctx context.Context, yield func(int) error) error {
			for i := range 40 {
				if err := yield(i); err != nil {
					return err
				}
			}
			return nil
		},
		func(ctx context.Context, i int) (int, error) {
			if n := outstanding.Add(1); n > 4 {
				t.Errorf("在途任务 %d", n)
			}
			if i == 0 {
				<-release
			}
			if i == 3 {
				close(release)
			}
			return i, nil
		},
		func(i int) error {
			if i != next {
				t.Errorf("收到 %d，期望 %d", i, next)
			}
			next++
			outstanding.Add(-1)
			return nil
		},
	)
	if err != nil || next != 40 {
		t.Fatal(next, err)
	}
}

// 读取失败之前已经交付的输入仍按顺序消费，读取错误最终返回给调用方。
func TestProducerFailure(t *testing.T) {
	failure := errors.New("source")
	count := 0
	err := Run(context.Background(), 3,
		func(ctx context.Context, yield func(int) error) error {
			for i := range 5 {
				if err := yield(i); err != nil {
					return err
				}
			}
			return failure
		},
		func(ctx context.Context, i int) (int, error) { return i, nil },
		func(i int) error { count++; return nil },
	)
	if !errors.Is(err, failure) || count != 5 {
		t.Fatal(count, err)
	}
}

// 无论取消还是读写计算失败，返回前都不能再有任务访问借用的资源。
func TestFailureWaitsForWorkers(t *testing.T) {
	for _, stage := range []string{"work", "consume", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := make(chan struct{})
			var started, active atomic.Int32
			var producerDone atomic.Bool
			failure := errors.New(stage)
			err := Run(ctx, 4,
				func(ctx context.Context, yield func(int) error) error {
					defer producerDone.Store(true)
					for i := 0; ; i++ {
						if err := yield(i); err != nil {
							return err
						}
					}
				},
				func(ctx context.Context, i int) (int, error) {
					active.Add(1)
					defer active.Add(-1)
					if started.Add(1) == 4 {
						close(ready)
					}
					<-ready
					if i == 0 {
						if stage == "work" {
							return i, failure
						}
						if stage == "cancel" {
							cancel()
						}
						return i, nil
					}
					<-ctx.Done()
					return i, ctx.Err()
				},
				func(i int) error { return failure },
			)
			want := failure
			if stage == "cancel" {
				want = context.Canceled
			}
			if !errors.Is(err, want) || active.Load() != 0 || !producerDone.Load() {
				t.Fatal(err, active.Load(), producerDone.Load())
			}
		})
	}
}

func TestMemoryBudgetLimitsConcurrency(t *testing.T) {
	got, err := Workers(64, 8, 16*1024*1024, 64*1024*1024)
	if err != nil || got != 1 {
		t.Fatal(got, err)
	}
	for _, pair := range [][2]int{{63, 1}, {512, -1}} {
		if _, err := Workers(pair[0], pair[1], 0, 1); err == nil {
			t.Fatal(pair)
		}
	}
}
