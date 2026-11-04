// Package java 把 Java 存档转换为 Bedrock。
//
// 和 bedrock 包命名一致：包名是输入版本，输出是另一个版本。
// 方块状态经 neomega-blocks 的 ConvertFromJava 转成 NEMC/Bedrock 状态；
// 本包负责调色板、含水层、群系和 diagnostics。
package java

import (
	"fmt"
	"maps"
	"sort"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter-core"
	"neomega-converter-core/java/blockentities"
	"neomega-converter-core/java/blockentities/banner"
	"neomega-converter-core/java/blockentities/chest"
	"neomega-converter-core/java/blockentities/furnace"
	"neomega-converter-core/java/blockentities/sign"
	tables "neomega-converter-core/third_party"
)

// Java 方块实体 id → Bedrock 规则。
var entityRules = map[string]blockentities.ConvertFunc{
	chest.ID:         chest.Convert,
	chest.TrappedID:  chest.Convert,
	furnace.ID:       furnace.Convert,
	furnace.BlastID:  furnace.Convert,
	furnace.SmokerID: furnace.Convert,
	banner.ID:        banner.Convert,
	sign.ID:          sign.Convert,
	sign.HangingID:   sign.Convert,
}

// Bedrock 1.21.50 的方块状态版本编码：1 << 24 | 21 << 16 | 50 << 8。
// 目标版本比它旧的游戏会升级调色板条目；不要写成比目标游戏新的版本。
const bedrockBlockVersion int32 = 1<<24 | 21<<16 | 50<<8

// 目标 subchunk 版本；celler 的 Encode 支持 1、8、9。
const sectionVersion byte = 9

var (
	airBlock   = cellerbedrock.Block{State: ir.IRBlock("minecraft:air []"), Version: bedrockBlockVersion}
	waterBlock = cellerbedrock.Block{
		State:   ir.IRBlock(`minecraft:water ["liquid_depth"=0]`),
		Version: bedrockBlockVersion,
	}
)

// ConvertChunk 把一根 Java 柱子转成 Bedrock 中间结果。
func ConvertChunk(pos celler.ChunkPos, dimension int32, src *cellerjava.Chunk) (*converter.BedrockChunk, error) {
	result := &converter.BedrockChunk{
		Pos:       pos,
		Dimension: dimension,
		Sections:  make(map[int32]*converter.BedrockSection),
	}
	ys, err := src.SectionYs()
	if err != nil {
		return nil, err
	}
	sort.Slice(ys, func(i, j int) bool { return ys[i] < ys[j] })

	unknown := make(map[ir.IRBlock]uint64)
	sources := make(map[int32]*cellerjava.Section, len(ys))
	for _, y := range ys {
		section, err := src.Section(y)
		if err != nil {
			return nil, fmt.Errorf("section %d: %w", y, err)
		}
		sources[y] = section
		// 只有光照的 section 没有方块调色板，不需要写出。
		if section.Blocks == nil {
			continue
		}
		result.Sections[y] = convertSection(section, unknown)
	}
	convertBlockEntities(src, result, sources, dimension)

	// 未知状态按名称排序，保证重复转换得到相同诊断顺序。
	states := make([]ir.IRBlock, 0, len(unknown))
	for state := range unknown {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return states[i] < states[j] })
	for _, state := range states {
		result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
			Kind:  converter.UnknownBlock,
			State: state,
			Count: unknown[state],
			Err:   fmt.Errorf("unknown Java block: %s", state),
		})
	}
	return result, nil
}

// convertBlockEntities 把 Java 的 block_entities 列表转成 Bedrock 方块实体。
func convertBlockEntities(src *cellerjava.Chunk, result *converter.BedrockChunk, sources map[int32]*cellerjava.Section, dimension int32) {
	documents, issues, err := src.BlockEntities()
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{Kind: converter.NBTError, Err: err})
		return
	}
	for _, issue := range issues {
		result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{Kind: converter.RuleError, Err: issue.Err})
	}
	unsupported := make(map[string]int)
	for _, document := range documents {
		pos := document.Pos
		id, _ := document.NBT["id"].(string)
		rule, ok := entityRules[id]
		if !ok {
			index, exists := unsupported[id]
			if !exists {
				index = len(result.Diagnostics)
				unsupported[id] = index
				result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
					Kind:          converter.UnsupportedBlockEntity,
					BlockEntityID: id,
					Err:           fmt.Errorf("no Java block entity rule for %q", id),
				})
			}
			result.Diagnostics[index].Count++
			continue
		}
		section := sources[pos.Y>>4]
		if section == nil || section.Blocks == nil {
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind: converter.RuleError, Pos: &pos,
				Err: fmt.Errorf("block entity is not in a converted section"),
			})
			continue
		}
		local := layer_storage.At(pos.X, pos.Y, pos.Z)
		javaState := section.Blocks.Get(local)
		base, ok, _ := convertBlock(javaState)
		if !ok {
			// 未知方块已经计入 unknown_block，不再重复报实体规则错误。
			continue
		}
		output, err := rule(blockentities.Input{
			SourceState: javaState,
			Base:        base.State,
			Source:      document.NBT,
			WorldPos:    pos,
			Dimension:   dimension,
			ReportItem:  result.ReportUnknownItem,
		})
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind: converter.RuleError, Pos: &pos, State: base.State, Err: err,
			})
			continue
		}
		state := output.State
		if state == "" {
			state = base.State
		}
		if state != base.State {
			if target := result.Sections[pos.Y>>4]; target != nil && len(target.Layers) > 0 {
				target.Layers[0].Set(local, cellerbedrock.Block{State: state.Tidy(), Version: bedrockBlockVersion})
			}
		}
		if output.BlockEntity != nil {
			value := maps.Clone(output.BlockEntity)
			value["x"], value["y"], value["z"] = pos.X, pos.Y, pos.Z
			result.BlockEntities = append(result.BlockEntities, value)
		}
	}
}

func convertSection(section *cellerjava.Section, unknown map[ir.IRBlock]uint64) *converter.BedrockSection {
	palette := section.Blocks.GetUsedPalette()
	// 先数每个源调色板条目占了多少格，未知方块按格数上报。
	counts := make([]uint64, len(palette))
	for index := range 4096 {
		counts[section.Blocks.GetIndex(layer_storage.At(index>>8, index&15, index>>4&15))]++
	}

	converted := make([]cellerbedrock.Block, len(palette))
	waterlogged := make([]bool, len(palette))
	hasWater := false
	for index, state := range palette {
		block, ok, wet := convertBlock(state)
		converted[index] = block
		waterlogged[index] = wet
		hasWater = hasWater || wet
		if !ok && counts[index] > 0 {
			unknown[state] += counts[index]
		}
	}
	base := &layer_storage.Palette[cellerbedrock.Block]{
		Data:           append([]uint32(nil), section.Blocks.Data...),
		Palette:        converted,
		Level:          section.Blocks.Level,
		IdxsPerUint32:  section.Blocks.IdxsPerUint32,
		Mask:           section.Blocks.Mask,
		LastPaletteIdx: section.Blocks.LastPaletteIdx,
	}
	layers := []*layer_storage.Palette[cellerbedrock.Block]{base}
	if hasWater {
		// Bedrock 用第二层水表示 Java 的 waterlogged；不是每个 section 都有。
		water := layer_storage.MakePaletteStorage(airBlock)
		for index := range 4096 {
			pos := layer_storage.At(index>>8, index&15, index>>4&15)
			if waterlogged[section.Blocks.GetIndex(pos)] {
				water.Set(pos, waterBlock)
			}
		}
		layers = append(layers, water)
	}
	return &converter.BedrockSection{Layers: layers, Biomes: convertBiomes(section.Biomes)}
}

// convertBlock 去掉 Java 的 waterlogged 扩展属性，再转成 Bedrock 状态。
// 未知状态返回空气；第二个返回值表示是否转换成功，第三个表示含水。
func convertBlock(state ir.IRBlock) (block cellerbedrock.Block, ok bool, waterlogged bool) {
	stripped, wet := stripWaterlogged(state)
	bedrock, ok := converts.ConvertFromJava(stripped)
	if !ok {
		return airBlock, false, wet
	}
	return cellerbedrock.Block{State: bedrock.Tidy(), Version: bedrockBlockVersion}, true, wet
}

func stripWaterlogged(state ir.IRBlock) (ir.IRBlock, bool) {
	value, exists := state.States().Get("waterlogged")
	if !exists {
		return state, false
	}
	wet := false
	switch value := value.(type) {
	case string:
		wet = value == "true"
	case bool:
		wet = value
	case int8:
		wet = value != 0
	}
	stripped := ir.IRBlock(state.NameWithNameSpace() + " " + string(state.States().Delete("waterlogged"))).Tidy()
	return stripped, wet
}

// convertBiomes 把 Java 的 4×4×4 群系样本复制成 Bedrock 的 16³ 数组。
// 没有群系字段的 section 全部写 plains。
func convertBiomes(biomes *[64]string) *layer_storage.Palette[int32] {
	palette := layer_storage.MakePaletteStorage(bedrockBiome("minecraft:plains"))
	if biomes == nil {
		return palette
	}
	for y4 := 0; y4 < 4; y4++ {
		for z4 := 0; z4 < 4; z4++ {
			for x4 := 0; x4 < 4; x4++ {
				id := bedrockBiome(biomes[y4<<4|z4<<2|x4])
				for dy := 0; dy < 4; dy++ {
					for dz := 0; dz < 4; dz++ {
						for dx := 0; dx < 4; dx++ {
							palette.Set(layer_storage.At(x4*4+dx, y4*4+dy, z4*4+dz), id)
						}
					}
				}
			}
		}
	}
	return palette
}

func bedrockBiome(java string) int32 {
	if id, ok := tables.BedrockBiome(java); ok {
		return id
	}
	if id, ok := tables.BedrockBiome("minecraft:plains"); ok {
		return id
	}
	return 1
}
