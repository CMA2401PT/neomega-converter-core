package structure

import "neomega-converter-core/internal/pipeline"

// Progress 描述当前阶段的工作量。Total 为零表示该阶段无法预先计数。
// reading 按压缩源文件字节计数，writing 按片区计数，extracting 按 zip 条目计数；
// 这些都是阶段进度，不是按耗时加权的整体百分比。
type Progress struct {
	Stage          string  `json:"stage"`
	Completed      int64   `json:"completed"`
	Total          int64   `json:"total"`
	ElapsedSeconds float64 `json:"elapsedSeconds"`
}

// Options 控制转换的内存预算、结构写出并发数与同步进度通知。
type Options struct {
	// MemoryMiB 是映射与并行工作的预算；零值为 512，至少 64。不是进程 RSS 硬上限。
	MemoryMiB int
	// Workers 是结构写出并发数的上限；零值按 CPU 自动选择，仍受内存预算约束。
	Workers int
	// Progress 由转换调用协程串行通知；回调应尽快返回。
	Progress func(Progress)
}

// withDefaults 补上零值选项，调用方只需要写关心的字段。
func (o Options) withDefaults() Options {
	if o.MemoryMiB == 0 {
		o.MemoryMiB = pipeline.DefaultMemoryMiB
	}
	return o
}
