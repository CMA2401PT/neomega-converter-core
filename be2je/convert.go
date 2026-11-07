package be2je

import (
	"fmt"
	"maps"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter-core"
	"neomega-converter-core/be2je/actors"
	"neomega-converter-core/be2je/blockentities"
	"neomega-converter-core/be2je/blockentities/banner"
	"neomega-converter-core/be2je/blockentities/beacon"
	"neomega-converter-core/be2je/blockentities/bed"
	"neomega-converter-core/be2je/blockentities/brewing_stand"
	"neomega-converter-core/be2je/blockentities/campfire"
	"neomega-converter-core/be2je/blockentities/chest"
	"neomega-converter-core/be2je/blockentities/command_block"
	"neomega-converter-core/be2je/blockentities/container"
	"neomega-converter-core/be2je/blockentities/decorated_pot"
	"neomega-converter-core/be2je/blockentities/end_gateway"
	"neomega-converter-core/be2je/blockentities/flower_pot"
	"neomega-converter-core/be2je/blockentities/furnace"
	"neomega-converter-core/be2je/blockentities/item_frame"
	"neomega-converter-core/be2je/blockentities/jukebox"
	"neomega-converter-core/be2je/blockentities/lectern"
	"neomega-converter-core/be2je/blockentities/mob_spawner"
	"neomega-converter-core/be2je/blockentities/note_block"
	"neomega-converter-core/be2je/blockentities/shulker_box"
	"neomega-converter-core/be2je/blockentities/sign"
	"neomega-converter-core/be2je/blockentities/skull"
	"neomega-converter-core/be2je/internal/blockstate"

	nbt "neomega-nbt/presets/v1"
)

var entityRules = map[string]blockentities.ConvertFunc{
	// 注册表由调用方持有，让规则子包只依赖共享的输入输出类型。
	bed.ID:            bed.Convert,
	flower_pot.ID:     flower_pot.Convert,
	banner.ID:         banner.Convert,
	shulker_box.ID:    shulker_box.Convert,
	jukebox.ID:        jukebox.Convert,
	lectern.ID:        lectern.Convert,
	note_block.ID:     note_block.Convert,
	skull.ID:          skull.Convert,
	chest.ID:          chest.Convert,
	sign.ID:           sign.Convert,
	sign.HangingID:    sign.Convert,
	item_frame.ID:     item_frame.Convert,
	item_frame.GlowID: item_frame.Convert,
	// 容器类方块实体先统一物品、名称和战利品表，再按类型补专属字段。
	"Barrel":         container.Convert("minecraft:barrel"),
	"Hopper":         container.Convert("minecraft:hopper"),
	"Dispenser":      container.Convert("minecraft:dispenser"),
	"Dropper":        container.Convert("minecraft:dropper"),
	"EnderChest":     container.Convert("minecraft:ender_chest"),
	furnace.ID:       furnace.Convert,
	furnace.BlastID:  furnace.Convert,
	furnace.SmokerID: furnace.Convert,
	brewing_stand.ID: brewing_stand.Convert,
	beacon.ID:        beacon.Convert,
	mob_spawner.ID:   mob_spawner.Convert,
	campfire.ID:      campfire.Convert,
	command_block.ID: command_block.Convert,
	end_gateway.ID:   end_gateway.Convert,
	decorated_pot.ID: decorated_pot.Convert,
}

const air ir.IRBlock = "minecraft:air []"

type blockResult struct {
	// 成功标记与状态一起缓存，未知方块的实际格数由调色板索引另行统计。
	state  ir.IRBlock
	source ir.IRBlock
	ok     bool
}

func ConvertState(source ir.IRBlock) (ir.IRBlock, bool) {
	// 单状态入口沿用内部的版本对齐与转出逻辑，和整根转换保持一致。
	return blockstate.ToJava(source)
}

func ConvertBlockEntity(id string, input blockentities.Input) (result blockentities.Result, ok bool, err error) {
	// 是否注册与规则执行失败分开返回，调用方可以区分缺规则和坏数据。
	rule, ok := entityRules[id]
	if !ok {
		return blockentities.Result{}, false, nil
	}
	result, err = rule(input)
	return result, true, err
}

func ConvertSection(
	pos celler.ChunkPos,
	dimension int32,
	section *cellerbedrock.Section,
	blockEntities []nbt.Compound,
) *converter.JavaChunk {
	// 已解码的数据直接进入阶段 1，实体的坐标范围仍由共用规则检查。
	return convertSections(
		pos,
		dimension,
		[]*cellerbedrock.Section{section},
		blockEntities,
	)
}

func ConvertChunk(src *cellerbedrock.Chunk) (*converter.JavaChunk, error) {
	return convertChunk(src, nil, nil)
}

// ConvertChunkWithActors 追加调用方从 digp/actorprefix 取得的实体。
// 0x32 记录仍由 ConvertChunk 自己读取。
func ConvertChunkWithActors(src *cellerbedrock.Chunk, extra []nbt.Compound) (*converter.JavaChunk, error) {
	return convertChunk(src, extra, nil)
}

// StateFallback is an optional preview mapping for states outside the export
// preset. It is never used by the default world export entry points.
// Return a dry Java state; the converter applies Bedrock's second water layer.
// Callbacks shared by adapt readers must be safe for concurrent use.
type StateFallback func(ir.IRBlock) (ir.IRBlock, bool)

// ConvertChunkWithStateFallback calls fallback only for unrecognized palette
// states. A nil or rejecting fallback preserves normal unknown-block diagnostics.
func ConvertChunkWithStateFallback(src *cellerbedrock.Chunk, extra []nbt.Compound, fallback StateFallback) (*converter.JavaChunk, error) {
	return convertChunk(src, extra, fallback)
}

func convertChunk(src *cellerbedrock.Chunk, extra []nbt.Compound, fallback StateFallback) (*converter.JavaChunk, error) {
	// 解码完整成功后才进入转换，任一 section 损坏仍使整根柱子失败。
	var sections []*cellerbedrock.Section
	for _, y := range src.SectionYs() {
		section, err := src.Section(y)
		if err != nil {
			return nil, fmt.Errorf("section %d: %w", y, err)
		}
		sections = append(sections, section)
	}
	documents, err := src.BlockEntities()
	var entities []nbt.Compound
	if err == nil {
		for _, document := range documents {
			entities = append(entities, document.Value)
		}
	}
	result := convertSections(
		src.Pos,
		src.Dimension,
		sections,
		entities,
		fallback,
	)
	applyBiomes(src, result)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{Kind: converter.NBTError, Err: err})
	}
	list, actorErr := chunkActors(src)
	if actorErr != nil {
		result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{Kind: converter.NBTError, Err: actorErr})
	}
	convertActors(result, append(list, extra...))
	return result, nil
}

// chunkActors 读取 0x32 记录；缺失表示这一柱没有旧式实体记录。
func chunkActors(src *cellerbedrock.Chunk) ([]nbt.Compound, error) {
	data, ok := src.Records[string([]byte{cellerbedrock.TagEntities})]
	if !ok {
		return nil, nil
	}
	documents, err := nbt.DecodeDocuments(data, nbt.EncodingLittleEndian)
	if err != nil {
		return nil, fmt.Errorf("entity record: %w", err)
	}
	list := make([]nbt.Compound, 0, len(documents))
	for _, document := range documents {
		list = append(list, document.Value)
	}
	return list, nil
}

func convertActors(result *converter.JavaChunk, list []nbt.Compound) {
	// 未知标识按名称合并计数；已知但跳过或数据不完整的实体不算错误。
	unknown := make(map[string]int)
	for _, source := range list {
		entity, known := actors.ConvertWithReport(source, result.ReportUnknownItem)
		if entity != nil {
			result.Entities = append(result.Entities, entity)
			continue
		}
		if known {
			continue
		}
		id, _ := source["identifier"].(string)
		index, exists := unknown[id]
		if !exists {
			index = len(result.Diagnostics)
			unknown[id] = index
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind:     converter.UnknownEntity,
				EntityID: id,
				Err:      fmt.Errorf("unknown Bedrock entity: %q", id),
			})
		}
		result.Diagnostics[index].Count++
	}
}

func convertSections(
	pos celler.ChunkPos,
	dimension int32,
	sections []*cellerbedrock.Section,
	entities []nbt.Compound,
	fallbacks ...StateFallback,
) *converter.JavaChunk {
	// 两个公开入口共用调色板、实体和第二层水的顺序，避免单 section 行为漂移。
	result := &converter.JavaChunk{
		Pos:       pos,
		Dimension: dimension,
		Sections:  make(map[int32]*converter.JavaSection),
	}
	cache := make(map[cellerbedrock.Block]blockResult)
	convert := func(source cellerbedrock.Block) ir.IRBlock {
		if cached, exists := cache[source]; exists {
			return cached.state
		}
		state, aligned, ok := blockstate.ToJavaWithSource(source.State)
		if !ok && len(fallbacks) != 0 && fallbacks[0] != nil {
			state, ok = fallbacks[0](source.State)
			aligned = source.State
		}
		if !ok {
			state = air
		}
		cache[source] = blockResult{state: state, source: aligned, ok: ok}
		return state
	}
	sources := make(map[int32]*cellerbedrock.Section)
	unknowns := make(map[ir.IRBlock]int)
	for _, section := range sections {
		y := section.Y
		sources[y] = section
		blocks := layer_storage.MakePaletteStorage(air)
		if len(section.Layers) > 0 {
			layer := section.Layers[0].Blocks
			blocks = layer_storage.ConvertPalette(layer, convert)
			counts := make([]uint64, len(layer.GetUsedPalette()))
			for index := range 4096 {
				counts[layer.GetIndex(layer_storage.At(index>>8, index&15, index>>4&15))]++
			}
			for index, source := range layer.GetUsedPalette() {
				if cache[source].ok || counts[index] == 0 {
					continue
				}
				diagnostic, exists := unknowns[source.State]
				if !exists {
					diagnostic = len(result.Diagnostics)
					unknowns[source.State] = diagnostic
					result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
						Kind:  converter.UnknownBlock,
						State: source.State,
						Err:   fmt.Errorf("unknown Bedrock block: %s", source.State),
					})
				}
				result.Diagnostics[diagnostic].Count += counts[index]
			}
		}
		result.Sections[y] = &converter.JavaSection{Blocks: blocks, Biomes: plainsBiomes}
	}
	entityCells := convertEntities(
		entities,
		result,
		sources,
		cache,
	)
	for _, section := range sections {
		y := section.Y
		if len(section.Layers) > 1 {
			applyWater(section, result.Sections[y].Blocks, entityCells[y], cache)
		}
	}
	return result
}

func convertEntities(
	entities []nbt.Compound,
	result *converter.JavaChunk,
	sources map[int32]*cellerbedrock.Section,
	cache map[cellerbedrock.Block]blockResult,
) map[int32]map[layer_storage.Position]bool {
	// 实体规则依赖单格 NBT，改过的格子单独标记，避免混入层组合的调色板缓存。
	changed := make(map[int32]map[layer_storage.Position]bool)
	unsupported := make(map[string]int)
	for _, entity := range entities {
		x, xOK := entity["x"].(int32)
		y, yOK := entity["y"].(int32)
		z, zOK := entity["z"].(int32)
		world := celler.BlockPos{X: x, Y: y, Z: z}
		section := result.Sections[y>>4]
		if !xOK || !yOK || !zOK || world.Chunk() != result.Pos || section == nil {
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind: converter.RuleError,
				Err:  fmt.Errorf("block entity requires Int x/y/z in a decoded section of its chunk"),
			})
			continue
		}
		local := layer_storage.At(x, y, z)
		base := section.Blocks.Get(local)
		id, ok := entity["id"].(string)
		if !ok {
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind: converter.RuleError, Pos: &world, State: base,
				Err: fmt.Errorf("block entity requires string id"),
			})
			continue
		}
		source := sources[y>>4]
		aligned := air
		if len(source.Layers) > 0 {
			cached := cache[source.Layers[0].Blocks.Get(local)]
			if !cached.ok {
				continue
			}
			aligned = cached.source
		}
		rule := entityRules[id]
		if rule == nil {
			diagnostic, exists := unsupported[id]
			if !exists {
				diagnostic = len(result.Diagnostics)
				unsupported[id] = diagnostic
				result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
					Kind:          converter.UnsupportedBlockEntity,
					BlockEntityID: id,
					Err:           fmt.Errorf("no Bedrock block entity rule for %q", id),
				})
			}
			result.Diagnostics[diagnostic].Count++
			continue
		}
		output, err := rule(blockentities.Input{
			Base:        base,
			Source:      entity,
			LocalPos:    [3]uint8{local.X, local.Y, local.Z},
			WorldPos:    world,
			SourceState: aligned,
			Dimension:   result.Dimension,
			ReportItem:  result.ReportUnknownItem,
		})
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind: converter.RuleError, Pos: &world, State: base, Err: err,
			})
			continue
		}
		section.Blocks.Set(local, output.State.Tidy())
		if changed[y>>4] == nil {
			changed[y>>4] = make(map[layer_storage.Position]bool)
		}
		changed[y>>4][local] = true
		if output.Entity != nil {
			result.Entities = append(result.Entities, output.Entity)
		}
		if output.BlockEntity != nil {
			value := maps.Clone(output.BlockEntity)
			value["x"], value["y"], value["z"] = x, y, z
			result.BlockEntities = append(result.BlockEntities, cellerjava.BlockEntity{Pos: world, NBT: value})
		}
	}
	return changed
}

func applyWater(
	source *cellerbedrock.Section,
	blocks *layer_storage.Palette[ir.IRBlock],
	entityCells map[layer_storage.Position]bool,
	cache map[cellerbedrock.Block]blockResult,
) {
	// 只缓存实际出现的层组合；实体规则产出的状态逐格附加含水属性。
	base, overlay := source.Layers[0].Blocks, source.Layers[1].Blocks
	combinations := make(map[[2]int]ir.IRBlock)
	for index := range 4096 {
		pos := layer_storage.At(index>>8, index&15, index>>4&15)
		if !cache[base.Get(pos)].ok {
			continue
		}
		name := overlay.Get(pos).State.NameWithNameSpace()
		if name != "minecraft:water" && name != "minecraft:flowing_water" {
			continue
		}
		key := [2]int{base.GetIndex(pos), overlay.GetIndex(pos)}
		state, exists := combinations[key]
		if !exists || entityCells[pos] {
			state = blocks.Get(pos)
			state = ir.IRBlock(state.NameWithNameSpace() + " " + string(state.States().Set("waterlogged", "true"))).Tidy()
			if !entityCells[pos] {
				combinations[key] = state
			}
		}
		blocks.Set(pos, state)
	}
}
