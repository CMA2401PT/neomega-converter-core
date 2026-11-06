package structure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Result 说明实际写出的存档种类与位置，viewer 按 Edition 选择原生存档读取器。
type Result struct {
	Directory string
	Name      string
	Edition   string
	// Chunks 是生成的 Java 柱子数；原生 Bedrock 路径不扫描计数。
	Chunks int
}

// FileKind 返回支持的结构格式；空字符串表示不是本转换器的输入。
func FileKind(path string) string {
	switch kind := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")); kind {
	case "schem", "schematic", "litematic", "nbt", "mcstructure", "mcworld":
		return kind
	default:
		return ""
	}
}

// WorldDirectory 返回与源文件同路径、去掉最后一个扩展名的存档目录。
func WorldDirectory(path string) string { return strings.TrimSuffix(path, filepath.Ext(path)) }

// ConvertFile 流式读取结构并生成原生存档。输出为空时使用源文件旁的同名目录。
// 已有非空目录不会覆盖；转换成功前只写临时目录，失败不留下半成品世界。
func ConvertFile(ctx context.Context, input, output string, options ...Options) (Result, error) {
	started := time.Now()
	var last time.Time
	previousStage := ""
	report := func(progress Progress) {
		if len(options) == 0 || options[0].Progress == nil {
			return
		}
		now := time.Now()
		if progress.Stage == previousStage && progress.Completed != progress.Total && now.Sub(last) < 200*time.Millisecond {
			return
		}
		previousStage, last = progress.Stage, now
		progress.ElapsedSeconds = now.Sub(started).Seconds()
		options[0].Progress(progress)
	}
	result := Result{Name: strings.TrimSuffix(filepath.Base(input), filepath.Ext(input)), Edition: "java"}
	kind := FileKind(input)
	if kind == "" {
		return result, fmt.Errorf("不支持的结构格式：%s", filepath.Ext(input))
	}
	if output == "" {
		output = WorldDirectory(input)
	}
	absolute, err := filepath.Abs(output)
	if err != nil {
		return result, err
	}
	result.Directory = absolute
	if kind == "mcstructure" || kind == "mcworld" {
		result.Edition = "bedrock"
	}
	// 检查发生在解析之前，避免大文件转换完成后才发现同名存档。
	entries, err := os.ReadDir(absolute)
	if err == nil && len(entries) > 0 {
		return result, fmt.Errorf("输出目录已有内容：%s", absolute)
	}
	if err != nil && !os.IsNotExist(err) {
		return result, err
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
		return result, err
	}
	working, err := os.MkdirTemp(filepath.Dir(absolute), ".structure-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(working)
	outputDir := filepath.Join(working, "world")
	if err = os.Mkdir(outputDir, 0755); err != nil {
		return result, err
	}
	var sink *blockSpool
	if result.Edition == "java" {
		sink, err = newBlockSpool(ctx, working)
		if err != nil {
			return result, err
		}
		defer sink.file.Close()
		sink.progress = report
	}
	var spawn [3]int32
	switch kind {
	case "schem":
		spawn, err = streamSchemFile(input, working, sink)
	case "schematic":
		spawn, err = streamSchematicFile(input, working, sink)
	case "litematic":
		spawn, err = streamLitematicFile(input, working, sink)
	case "nbt":
		spawn, err = streamJavaNBTFile(input, working, sink)
	case "mcstructure":
		err = convertMCStructure(ctx, input, outputDir, result.Name, report)
	case "mcworld":
		err = extractMCWorld(ctx, input, outputDir, report)
	}
	if err != nil {
		return result, err
	}
	if result.Edition == "java" {
		result.Chunks, err = sink.write(outputDir, result.Name, spawn)
		if err != nil {
			return result, err
		}
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	report(Progress{Stage: "finishing"})
	// 仅移除先前确认过的空目录；非空目录在 Rename 时仍会失败。
	if err = os.Remove(absolute); err != nil && !os.IsNotExist(err) {
		return result, err
	}
	if err = os.Rename(outputDir, absolute); err != nil {
		return result, err
	}
	report(Progress{Stage: "complete", Completed: 1, Total: 1})
	return result, nil
}
