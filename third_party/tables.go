// Package tables 提供 Bedrock 与 Java 之间非方块数据的查表入口。
//
// 表由 third_party/gen 从上游生成并内嵌；调用方不需要知道数据来自哪里、版本是
// 什么。方块状态与旧状态升级规则仍由 neomega-blocks 维护，不在这里。
//
// 表是只读的：所有访问都从同一份内嵌数据解析一次，可以并发调用。
package tables

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

//go:embed data/tables.json.gz
var archive []byte

// Source 描述某一类数据的上游位置和许可。
type Source struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Ref     string            `json:"ref"`
	Commit  string            `json:"commit"`
	License string            `json:"license"`
	Files   map[string]string `json:"files"`
}

type itemEntry struct {
	Bedrock string `json:"bedrock"`
	Data    int32  `json:"data"`
	Java    string `json:"java"`
}

type biomeEntry struct {
	ID      int32  `json:"id"`
	Bedrock string `json:"bedrock"`
	Java    string `json:"java"`
}

type patternEntry struct {
	Code string `json:"code"`
	Java string `json:"java"`
}

type file struct {
	Generator      int            `json:"generator"`
	JavaVersion    string         `json:"java_version"`
	BedrockVersion string         `json:"bedrock_version"`
	Sources        []Source       `json:"sources"`
	Items          []itemEntry    `json:"items"`
	Biomes         []biomeEntry   `json:"biomes"`
	Patterns       []patternEntry `json:"patterns"`
	Entities       []string       `json:"entities"`
}

type itemKey struct {
	name string
	data int32
}

type dataset struct {
	file     file
	items    map[itemKey]string
	biomes   map[int32]string
	patterns map[string]string
	entities map[string]bool
}

var load = sync.OnceValue(func() *dataset {
	reader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		panic(fmt.Errorf("third_party: 打开内嵌表: %w", err))
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("third_party: 解压内嵌表: %w", err))
	}
	var parsed file
	if err := json.Unmarshal(decoded, &parsed); err != nil {
		panic(fmt.Errorf("third_party: 解析内嵌表: %w", err))
	}
	data := &dataset{
		file:     parsed,
		items:    make(map[itemKey]string, len(parsed.Items)),
		biomes:   make(map[int32]string, len(parsed.Biomes)),
		patterns: make(map[string]string, len(parsed.Patterns)),
		entities: make(map[string]bool, len(parsed.Entities)),
	}
	for _, entry := range parsed.Items {
		data.items[itemKey{name: entry.Bedrock, data: entry.Data}] = entry.Java
	}
	for _, entry := range parsed.Biomes {
		data.biomes[entry.ID] = entry.Java
	}
	for _, entry := range parsed.Patterns {
		data.patterns[entry.Code] = entry.Java
	}
	for _, name := range parsed.Entities {
		data.entities[name] = true
	}
	return data
})

// Versions 返回表对应的 Java 与 Bedrock 版本。
func Versions() (java string, bedrock string) {
	data := load()
	return data.file.JavaVersion, data.file.BedrockVersion
}

// Sources 返回上游来源的副本。
func Sources() []Source {
	file := load().file
	out := make([]Source, len(file.Sources))
	copy(out, file.Sources)
	return out
}

// Item 把 Bedrock 物品名与 data value 转成 Java 物品 id。
// 精确的 data value 没有条目时回退到 data 0，与 mcconv 的查表顺序一致。
func Item(bedrock string, data int32) (string, bool) {
	name := prefixed(bedrock)
	if name == "" {
		return "", false
	}
	table := load().items
	if java, ok := table[itemKey{name: name, data: data}]; ok {
		return java, true
	}
	java, ok := table[itemKey{name: name}]
	return java, ok
}

// Biome 把 Bedrock 生物群系数字 id 转成 Java 群系名。
func Biome(id int32) (string, bool) {
	java, ok := load().biomes[id]
	return java, ok
}

// BannerPattern 把 Bedrock 旗帜图案代码转成 Java 图案名。
func BannerPattern(code string) (string, bool) {
	java, ok := load().patterns[code]
	return java, ok
}

// KnownEntity 报告 Bedrock 是否有这个实体标识。
func KnownEntity(id string) bool {
	return load().entities[prefixed(id)]
}

func prefixed(name string) string {
	if name == "" {
		return ""
	}
	if strings.ContainsRune(name, ':') {
		return name
	}
	return "minecraft:" + name
}
