package structure

// Progress 描述当前阶段的工作量。Total 为零表示该阶段无法预先计数。
// reading 按压缩源文件字节计数，writing 按片区计数，extracting 按 zip 条目计数；
// 这些都是阶段进度，不是按耗时加权的整体百分比。
type Progress struct {
	Stage          string  `json:"stage"`
	Completed      int64   `json:"completed"`
	Total          int64   `json:"total"`
	ElapsedSeconds float64 `json:"elapsedSeconds"`
}

// Options 为转换调用方提供同步进度通知；回调应尽快返回。
type Options struct {
	Progress func(Progress)
}
