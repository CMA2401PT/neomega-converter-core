package structure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Edition 是写出的存档版本。
type Edition string

const (
	// Java 结构保留 Java 状态，写成 Java 世界。
	Java Edition = "java"
	// Bedrock 结构保留原生双层状态，写成基岩世界。
	Bedrock Edition = "bedrock"
)

// Result 说明实际写出的存档种类与位置，viewer 按 Edition 选择原生存档读取器。
type Result struct {
	Directory string
	Name      string
	Edition   Edition
	// Chunks 是生成的 Java 柱子数；原生 Bedrock 路径不扫描计数。
	Chunks int
}

// format 是一种输入格式：写成哪个版本，以及把文件读进暂存目录的函数。
type format struct {
	edition Edition
	convert func(*job) error
}

// formats 是支持的结构格式。schem、schematic、litematic 和原版 nbt 保留 Java 状态，
// mcstructure 和 mcworld 保留基岩状态。
var formats = map[string]format{
	"schem":       {Java, convertSchem},
	"schematic":   {Java, convertSchematic},
	"litematic":   {Java, convertLitematic},
	"nbt":         {Java, convertTemplate},
	"mcstructure": {Bedrock, convertMCStructure},
	"mcworld":     {Bedrock, convertMCWorld},
}

// FileKind 返回支持的结构格式；空字符串表示不是本转换器的输入。
func FileKind(path string) string {
	kind := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if _, ok := formats[kind]; ok {
		return kind
	}
	return ""
}

// WorldDirectory 返回与源文件同路径、去掉最后一个扩展名的存档目录。
func WorldDirectory(path string) string { return strings.TrimSuffix(path, filepath.Ext(path)) }

// job 是一次转换的上下文：输入、暂存目录、发布目标、进度和共用状态。
// convert 只写 staging；成功后 staging/world 才改名成 output。
type job struct {
	ctx     context.Context
	input   string
	staging string
	output  string
	name    string
	opts    Options
	report  func(Progress)
	// spool 只在 Java 结构下存在；Bedrock 结构按自己的布局直接写区块。
	spool *blockSpool
	// spawn 是 convert 算出的出生点，写出存档时使用。
	spawn [3]int32
}

// ConvertFile 流式读取结构并生成原生存档。输出为空时使用源文件旁的同名目录。
// 已有非空目录不会覆盖；转换成功前只写临时目录，失败不留下半成品世界。
func ConvertFile(ctx context.Context, input, output string, options ...Options) (Result, error) {
	opts := Options{}
	if len(options) > 0 {
		opts = options[0]
	}
	opts = opts.withDefaults()
	if opts.MemoryMiB < 64 || opts.Workers < 0 {
		return Result{}, fmt.Errorf("结构转换内存预算至少 64 MiB，workers 不能为负数")
	}
	kind := FileKind(input)
	inputFormat, ok := formats[kind]
	if !ok {
		return Result{}, fmt.Errorf("不支持的结构格式：%s", filepath.Ext(input))
	}
	if output == "" {
		output = WorldDirectory(input)
	}
	absolute, err := filepath.Abs(output)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Directory: absolute,
		Name:      strings.TrimSuffix(filepath.Base(input), filepath.Ext(input)),
		Edition:   inputFormat.edition,
	}
	// 检查发生在解析之前，避免大文件转换完成后才发现同名存档。
	if entries, err := os.ReadDir(absolute); err == nil && len(entries) > 0 {
		return result, fmt.Errorf("输出目录已有内容：%s", absolute)
	} else if err != nil && !os.IsNotExist(err) {
		return result, err
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
		return result, err
	}
	staging, err := os.MkdirTemp(filepath.Dir(absolute), ".structure-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(staging)
	world := filepath.Join(staging, "world")
	if err = os.Mkdir(world, 0755); err != nil {
		return result, err
	}
	current := &job{
		ctx:     ctx,
		input:   input,
		staging: staging,
		output:  world,
		name:    result.Name,
		opts:    opts,
		report:  newReporter(opts.Progress, time.Now()),
	}
	if inputFormat.edition == Java {
		if current.spool, err = newBlockSpool(current); err != nil {
			return result, err
		}
		defer current.spool.discard()
	}
	if err = inputFormat.convert(current); err != nil {
		return result, err
	}
	if current.spool != nil {
		if result.Chunks, err = current.spool.writeWorld(current.output, current.name, current.spawn); err != nil {
			return result, err
		}
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	current.report(Progress{Stage: "finishing"})
	// 仅移除先前确认过的空目录；非空目录在 Rename 时仍会失败。
	if err = os.Remove(absolute); err != nil && !os.IsNotExist(err) {
		return result, err
	}
	if err = os.Rename(current.output, absolute); err != nil {
		return result, err
	}
	current.report(Progress{Stage: "complete", Completed: 1, Total: 1})
	return result, nil
}

// newReporter 返回带去重的进度回调：同一阶段 200 ms 内只转发一次，阶段切换和完成不节流。
// 没有回调时返回空函数，调用方不必再判空。
func newReporter(progress func(Progress), started time.Time) func(Progress) {
	if progress == nil {
		return func(Progress) {}
	}
	var last time.Time
	stage := ""
	return func(value Progress) {
		now := time.Now()
		if value.Stage == stage && value.Completed != value.Total && now.Sub(last) < 200*time.Millisecond {
			return
		}
		stage, last = value.Stage, now
		value.ElapsedSeconds = now.Sub(started).Seconds()
		progress(value)
	}
}
