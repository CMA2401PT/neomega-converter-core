// Command gen 从 GeyserMC/mappings 与 pmmp/BedrockData 生成
// third_party/data/tables.json.gz。
//
// 上游与 mcconv 的 tools/gendata 相同：GeyserMC 提供 Java↔Bedrock 的物品和
// 生物群系对应，pmmp 提供 Bedrock 群系编号、旗帜图案代码和实体名。这里只取
// 转换核心需要的四类数据；方块表与旧状态升级规则仍由 neomega-blocks 维护。
//
// 输入文件、上游版本和 sha256 固定在 sources.lock.json，生成前逐一核对。
// 输出按名称排序并固定 gzip 头，同样输入得到同样字节。
//
// 在工作区中从模块目录运行：
//
//	go run ./third_party/gen -geyser <checkout> -pmmp <checkout>
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	converts "neomega-blocks/presets/block-converts/v1"
)

// 锁文件描述生成所需的每个上游文件。读取时先核对 sha256，避免悄悄换版本。
type lockFile struct {
	JavaVersion    string       `json:"java_version"`
	BedrockVersion string       `json:"bedrock_version"`
	Sources        []lockSource `json:"sources"`
}

type lockSource struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Ref     string            `json:"ref"`
	Commit  string            `json:"commit"`
	License string            `json:"license"`
	Files   map[string]string `json:"files"`
}

type sourceInfo struct {
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

// 反向索引：Java → Bedrock。表里同时保存两个方向，调用方不用自己反查。

type javaItemEntry struct {
	Java    string `json:"java"`
	Bedrock string `json:"bedrock"`
	Data    int32  `json:"data"`
}

type javaBiomeEntry struct {
	Java string `json:"java"`
	ID   int32  `json:"id"`
}

type javaPatternEntry struct {
	Java string `json:"java"`
	Code string `json:"code"`
}

type table struct {
	Generator      int                `json:"generator"`
	JavaVersion    string             `json:"java_version"`
	BedrockVersion string             `json:"bedrock_version"`
	Sources        []sourceInfo       `json:"sources"`
	Stats          stats              `json:"stats"`
	Items          []itemEntry        `json:"items"`
	Biomes         []biomeEntry       `json:"biomes"`
	Patterns       []patternEntry     `json:"patterns"`
	Entities       []string           `json:"entities"`
	JavaItems      []javaItemEntry    `json:"java_items"`
	JavaBiomes     []javaBiomeEntry   `json:"java_biomes"`
	JavaPatterns   []javaPatternEntry `json:"java_patterns"`
}

type stats struct {
	JavaItems           int `json:"java_items"`
	ItemKeys            int `json:"item_keys"`
	ItemCollisions      int `json:"item_collisions"`
	BlockItems          int `json:"block_items"`
	BlockItemMismatches int `json:"block_item_mismatches"`
	BiomeCollisions     int `json:"biome_collisions"`
	Biomes              int `json:"biomes"`
	Patterns            int `json:"patterns"`
	Entities            int `json:"entities"`
	JavaItemIndex       int `json:"java_item_index"`
	JavaBiomeIndex      int `json:"java_biome_index"`
	JavaPatternIndex    int `json:"java_pattern_index"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	geyser := flag.String("geyser", "", "GeyserMC/mappings checkout（含 items.json、biomes.json）")
	pmmp := flag.String("pmmp", "", "pmmp/BedrockData checkout（含 biome_id_map.json、banner_patterns.json、entity_id_map.json）")
	lockPath := flag.String("lock", "third_party/sources.lock.json", "来源与校验和锁文件")
	out := flag.String("out", "third_party/data/tables.json.gz", "输出文件")
	flag.Parse()
	if *geyser == "" || *pmmp == "" {
		return errors.New("必须提供 -geyser 与 -pmmp")
	}
	lock, err := readLock(*lockPath)
	if err != nil {
		return err
	}
	directories := map[string]string{
		"GeyserMC/mappings": *geyser,
		"pmmp/BedrockData":  *pmmp,
	}
	// 一次读取并核对所有输入，后面直接按来源名取用。
	contents := map[string][]byte{}
	for _, source := range lock.Sources {
		directory, ok := directories[source.Name]
		if !ok {
			return fmt.Errorf("锁文件中有未知来源 %q", source.Name)
		}
		for name, want := range source.Files {
			path := filepath.Join(directory, name)
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			if got := hex.EncodeToString(sum[:]); got != want {
				return fmt.Errorf("%s 的 sha256 不匹配：得到 %s，锁文件为 %s", path, got, want)
			}
			contents[source.Name+"/"+name] = data
		}
	}

	items, itemStats, blockJava, err := buildItems(contents["GeyserMC/mappings/items.json"])
	if err != nil {
		return err
	}
	mismatches, blockItems, err := checkBlockItems(items, blockJava)
	if err != nil {
		return err
	}
	itemStats.BlockItems = blockItems
	itemStats.BlockItemMismatches = mismatches
	biomes, biomeCollisions, err := buildBiomes(
		contents["GeyserMC/mappings/biomes.json"],
		contents["pmmp/BedrockData/biome_id_map.json"],
	)
	if err != nil {
		return err
	}
	patterns, err := buildPatterns(contents["pmmp/BedrockData/banner_patterns.json"])
	if err != nil {
		return err
	}
	entities, err := buildEntities(contents["pmmp/BedrockData/entity_id_map.json"])
	if err != nil {
		return err
	}
	javaItems, err := buildJavaItems(contents["GeyserMC/mappings/items.json"])
	if err != nil {
		return err
	}
	javaBiomes, err := buildJavaBiomes(contents["GeyserMC/mappings/biomes.json"])
	if err != nil {
		return err
	}
	javaPatterns, err := buildJavaPatterns(contents["pmmp/BedrockData/banner_patterns.json"])
	if err != nil {
		return err
	}
	itemStats.BiomeCollisions = biomeCollisions

	tbl := table{
		Generator:      1,
		JavaVersion:    lock.JavaVersion,
		BedrockVersion: lock.BedrockVersion,
		Sources:        sources(lock),
		Stats:          itemStats,
		Items:          items,
		Biomes:         biomes,
		Patterns:       patterns,
		Entities:       entities,
		JavaItems:      javaItems,
		JavaBiomes:     javaBiomes,
		JavaPatterns:   javaPatterns,
	}
	tbl.Stats.Biomes = len(biomes)
	tbl.Stats.Patterns = len(patterns)
	tbl.Stats.Entities = len(entities)
	tbl.Stats.JavaItemIndex = len(javaItems)
	tbl.Stats.JavaBiomeIndex = len(javaBiomes)
	tbl.Stats.JavaPatternIndex = len(javaPatterns)

	if err := writeTable(*out, tbl); err != nil {
		return err
	}
	fmt.Printf("物品 %d（键 %d，冲突 %d），群系 %d（冲突 %d），图案 %d，实体 %d\n",
		tbl.Stats.JavaItems, tbl.Stats.ItemKeys, tbl.Stats.ItemCollisions,
		tbl.Stats.Biomes, tbl.Stats.BiomeCollisions, tbl.Stats.Patterns, tbl.Stats.Entities)
	return nil
}

func readLock(path string) (lockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return lockFile{}, err
	}
	var lock lockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return lockFile{}, fmt.Errorf("%s: %w", path, err)
	}
	if lock.JavaVersion == "" || lock.BedrockVersion == "" {
		return lockFile{}, fmt.Errorf("%s: 缺少 java_version 或 bedrock_version", path)
	}
	return lock, nil
}

func sources(lock lockFile) []sourceInfo {
	out := make([]sourceInfo, 0, len(lock.Sources))
	for _, source := range lock.Sources {
		out = append(out, sourceInfo{
			Name: source.Name, URL: source.URL, Ref: source.Ref,
			Commit: source.Commit, License: source.License, Files: source.Files,
		})
	}
	return out
}

type rawItem struct {
	BedrockIdentifier   string `json:"bedrock_identifier"`
	BedrockData         int32  `json:"bedrock_data"`
	FirstBlockRuntimeID *int32 `json:"firstBlockRuntimeId"`
}

type itemKey struct {
	name string
	data int32
}

// buildItems 建立 (Bedrock 名, data value) → Java 物品名的反查表。
// 同一键有多个 Java 候选时优先取与 Bedrock 同名的那个，与 mcconv 的规则一致；
// 其余情况按 Java 名排序取第一个，保证输出稳定。
func buildItems(data []byte) ([]itemEntry, stats, map[string]bool, error) {
	var raw map[string]rawItem
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, stats{}, nil, fmt.Errorf("items.json: %w", err)
	}
	javaIDs := make([]string, 0, len(raw))
	for java := range raw {
		javaIDs = append(javaIDs, java)
	}
	sort.Strings(javaIDs)

	candidates := map[itemKey][]string{}
	blockItems := map[string]bool{}
	for _, java := range javaIDs {
		item := raw[java]
		name := namespaced(item.BedrockIdentifier)
		if name == "" {
			return nil, stats{}, nil, fmt.Errorf("物品 %q 缺少 bedrock_identifier", java)
		}
		key := itemKey{name: name, data: item.BedrockData}
		candidates[key] = append(candidates[key], java)
		if item.FirstBlockRuntimeID != nil {
			blockItems[java] = true
		}
	}

	keys := make([]itemKey, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].data < keys[j].data
	})

	entries := make([]itemEntry, 0, len(keys))
	collisions := 0
	for _, key := range keys {
		list := candidates[key]
		java := list[0]
		for _, candidate := range list {
			if candidate == key.name {
				java = candidate
				break
			}
		}
		if len(list) > 1 {
			collisions++
		}
		entries = append(entries, itemEntry{Bedrock: key.name, Data: key.data, Java: java})
	}
	return entries, stats{
		JavaItems:      len(raw),
		ItemKeys:       len(entries),
		ItemCollisions: collisions,
	}, blockItems, nil
}

// checkBlockItems 用 Java 方块表交叉验证块物品，抓 Geyser 版本与方块表错位。
// 少数物品放置的方块与其物品名不同（种子、红石、可可豆等），允许 2% 误差。
func checkBlockItems(entries []itemEntry, blockJava map[string]bool) (mismatches, blockItems int, err error) {
	for _, entry := range entries {
		if !blockJava[entry.Java] {
			continue
		}
		blockItems++
		if converts.IsJavaBlock(entry.Java) {
			continue
		}
		mismatches++
		if mismatches <= 20 {
			fmt.Printf("  非方块名：%s/%d → %s\n", entry.Bedrock, entry.Data, entry.Java)
		}
	}
	if blockItems == 0 {
		return 0, 0, errors.New("items.json 中没有块物品，Geyser 版本可能不对")
	}
	if mismatches*50 > blockItems {
		return mismatches, blockItems, fmt.Errorf("块物品名称校验失败：%d/%d 不在 Java 方块表中", mismatches, blockItems)
	}
	return mismatches, blockItems, nil
}

type rawBiome struct {
	BedrockID int32 `json:"bedrock_id"`
}

// buildBiomes 建立 Bedrock 数字 id → Java 群系名。Geyser 直接给出两者的对应，
// pmmp 的名字只用于诊断与同候选时的优先规则。
func buildBiomes(geyserData, pmmpData []byte) ([]biomeEntry, int, error) {
	var geyser map[string]rawBiome
	if err := json.Unmarshal(geyserData, &geyser); err != nil {
		return nil, 0, fmt.Errorf("biomes.json: %w", err)
	}
	var pmmp map[string]int32
	if err := json.Unmarshal(pmmpData, &pmmp); err != nil {
		return nil, 0, fmt.Errorf("biome_id_map.json: %w", err)
	}
	names := make(map[int32]string, len(pmmp))
	for name, id := range pmmp {
		names[id] = name
	}
	byID := map[int32][]string{}
	for java, biome := range geyser {
		byID[biome.BedrockID] = append(byID[biome.BedrockID], java)
	}
	ids := make([]int32, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	entries := make([]biomeEntry, 0, len(ids))
	collisions := 0
	for _, id := range ids {
		list := byID[id]
		sort.Strings(list)
		java := list[0]
		for _, candidate := range list {
			if candidate == namespaced(names[id]) {
				java = candidate
				break
			}
		}
		if len(list) > 1 {
			collisions++
		}
		entries = append(entries, biomeEntry{ID: id, Bedrock: names[id], Java: java})
	}
	return entries, collisions, nil
}

type rawPattern struct {
	ID string `json:"id"`
}

// buildPatterns 建立 Bedrock 图案代码 → Java 图案名。
func buildPatterns(data []byte) ([]patternEntry, error) {
	var raw map[string]rawPattern
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("banner_patterns.json: %w", err)
	}
	entries := make([]patternEntry, 0, len(raw))
	for name, pattern := range raw {
		if pattern.ID == "" {
			continue
		}
		entries = append(entries, patternEntry{Code: pattern.ID, Java: namespaced(name)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Code < entries[j].Code })
	return entries, nil
}

// buildJavaItems 直接使用 Geyser 的 Java → Bedrock 对应，不反向推算，
// 因为正向表里的同键冲突是按名字偏好解决的，反推会丢失候选。
func buildJavaItems(data []byte) ([]javaItemEntry, error) {
	var raw map[string]rawItem
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("items.json: %w", err)
	}
	entries := make([]javaItemEntry, 0, len(raw))
	for java, item := range raw {
		entries = append(entries, javaItemEntry{
			Java: java, Bedrock: namespaced(item.BedrockIdentifier), Data: item.BedrockData,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Java < entries[j].Java })
	return entries, nil
}

func buildJavaBiomes(data []byte) ([]javaBiomeEntry, error) {
	var raw map[string]rawBiome
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("biomes.json: %w", err)
	}
	entries := make([]javaBiomeEntry, 0, len(raw))
	for java, biome := range raw {
		entries = append(entries, javaBiomeEntry{Java: java, ID: biome.BedrockID})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Java < entries[j].Java })
	return entries, nil
}

func buildJavaPatterns(data []byte) ([]javaPatternEntry, error) {
	var raw map[string]rawPattern
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("banner_patterns.json: %w", err)
	}
	entries := make([]javaPatternEntry, 0, len(raw))
	for name, pattern := range raw {
		if pattern.ID == "" {
			continue
		}
		entries = append(entries, javaPatternEntry{Java: namespaced(name), Code: pattern.ID})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Java < entries[j].Java })
	return entries, nil
}

func buildEntities(data []byte) ([]string, error) {
	var raw map[string]int32
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("entity_id_map.json: %w", err)
	}
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, namespaced(name))
	}
	sort.Strings(names)
	return names, nil
}

func namespaced(name string) string {
	if name == "" {
		return ""
	}
	for _, r := range name {
		if r == ':' {
			return name
		}
	}
	return "minecraft:" + name
}

func writeTable(path string, tbl table) error {
	encoded, err := json.Marshal(tbl)
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return err
	}
	// 固定头字段，重复生成得到相同字节。
	writer.Header.OS = 255
	if _, err := writer.Write(encoded); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("写入 %s（%d 字节，JSON %d 字节）\n", path, buffer.Len(), len(encoded))
	return nil
}
