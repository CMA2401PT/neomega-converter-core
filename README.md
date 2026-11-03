# neomega-converter-core

本模块将 Bedrock 方块、方块实体和存档转换为 Java 数据，供渲染或写出使用。
一根区块柱子的水平范围是 16×16，内部按高度分成 16×16×16 的 section。
转换只读取源柱子；邻居相关的属性修复由 [neomega-converter](../apps/neomega-converter/README.md) 组合。
本模块依赖方块、NBT 与存档基础库，不依赖渲染器或 fixer。

## 能力一览

本模块对外提供几层相互独立的能力，调用方按需要组合。
下面用伪代码列出每层的签名、支持到哪一步、什么时候报错，以及不报错但结果降级的情况。
「降级」指调用成功，但输出比源数据少了信息或只是近似。
所列行为以当前代码为准，修改实现时请同步这一节。

```text
能力                                   输入 → 输出
bedrock.ConvertState(state)            单个 Bedrock 状态 → Java 状态, ok
items.ConvertItem(item)                Bedrock 物品 compound → Java 物品 compound 或 nil
bedrock.ConvertBlockEntity(id, Input)  单格方块实体 → Java 状态 + 可选方块实体 / 实体, ok, err
bedrock.ConvertSection(pos, dim, ...)  单个 Bedrock section + 方块实体 → ConvertedChunk
bedrock.ConvertChunk(chunk)            Bedrock 整根柱子 → ConvertedChunk, err
adapt.NewConverted(read, opts)        按坐标读取 Bedrock 柱子 → 缓存转换结果 的来源
export.ToJavaChunk / ToJavaEntities    ConvertedChunk → Java 区块 / Java 实体记录
export.ConvertLevel(level)             Bedrock level.dat → Java level.dat
export.ConvertWorld(src, dst, opts)    Bedrock 存档 → Java 存档，不跑 fixer
```

这些入口都在本模块；`ConvertState` 内部使用 `neomega-blocks/presets/block-converts/v1` 的两个函数。
转换的三个粒度共用同一套实现：`ConvertSection` 和 `ConvertChunk` 只差在输入是已解码的单个 section 还是整根柱子的原始记录。

### 单个方块状态

存档里的状态可能来自任意 Bedrock 版本，而 Java 转出表只认目标版本的状态。
所以转换分两步：`ConvertFromBedrock` 先把旧方块名、旧属性和缺失的属性对齐到目标版本，`ConvertToJava` 再转成 Java。
`bedrock.ConvertState` 就是这两步，本模块的区块转换、花盆规则和物品转换都经过它。

```text
ConvertState(state.Tidy()) → (java, ok)
  内部：nemc = ConvertFromBedrock(state)；java = ConvertToJava(nemc)
  目标集合：NEMC 调色板（参考 Bedrock 1.21.50，共 15156 个状态），全部能转出
  输出：Java 1.21.4 的完整状态，已经 tidy
  ok=false：方块名不认识；或属性值不在取值域内，例如 weirdo_direction=99
  降级（ok=true）：
    缺少的属性补默认值        oak_stairs []          → 朝东的直楼梯
    179 个方块名映射成空气    教育版元素与化合物、hard_*glass、colored_torch_*、
                              underwater_torch、border_block、invisible_bedrock、micro_block、
                              旧版 PE 方块（glowingobsidian、netherreactor、info_update 等）
    frame / glow_frame 映射成空气，因为 Java 的展示框是实体，不是方块；实体由下文的 item_frame 规则产出
    颜色或内容在 NBT 里的方块只得到默认外观
                              bed → red_bed，standing_banner → white_banner，flower_pot → 空花盆
    输出不带 waterlogged
```

两个函数都不看调色板条目的版本号。
同一段状态文本在不同版本含义不同时，按生成转换表时的优先级解释，详见 neomega-blocks 的 README。

### items.ConvertItem：单个物品

```text
items.ConvertItem(bedrockItem) → javaItem 或 nil
  支持：方块物品，Java 物品 ID 取转出的 Java 方块名；变体按以下顺序确定
          1. 物品带 Block compound（name + states）时用它
          2. 否则按 Name + 顶层 Damage 查 legacy data value
          3. 都失败时只用 Name，取默认状态
        potion / splash_potion / lingering_potion、药水箭（arrow 且 Damage>0）
        written_book（标题、作者、代数、书页）、writable_book（书页原文）
        附魔 tag.ench → minecraft:enchantments；保留 Slot
  返回 nil，调用方跳过整个物品：
        名称为空或 air、Count<=0
        工具、武器、盔甲、食物、普通箭等非方块物品（缺物品名映射表）
        带 tag.Items 的嵌套容器
        药水编号越界或为 decay、附魔编号越界或字段类型错误、书页不是字符串
  降级：
        没有 Block compound 的旧物品，名称已不在目标版本时（如 wool + Damage 14）
          legacy data value 查不到，退回默认状态（white_wool）
        床、旗帜、头颅的 Damage 是颜色或种类，不是方块数据，退回默认外观
        不写耐久：目前支持的物品都没有耐久
        tag 中其它字段（自定义名称、lore、CanPlaceOn 等）丢弃
        § 格式码原样放进文本组件，不解析成样式
```

### 方块实体规则：单格

每个规则子包在 `bedrock/blockentities/` 下，导出 `ID`（Bedrock 实体 id）和 `Convert`。
`bedrock.ConvertBlockEntity` 按 id 从注册表选规则。

```text
ConvertBlockEntity(id, Input{Base, SourceState, Source, LocalPos, WorldPos, Dimension})
    → (Result{State, BlockEntity, Entity}, ok, err)
  Base 是已转换的 Java 状态，规则在它的基础上改名或改属性
  SourceState 是对齐到目标 Bedrock 版本的源状态，供 Java 状态里已经没有的属性使用（展示框朝向）
  ok=false：这个 id 没有规则
  err != nil：调用方应保留 Base，丢弃本条输出
```

| 子包 / ID | 改什么 | 报错 | 降级 |
|---|---|---|---|
| bed / `Bed` | 颜色进方块名 | color 不是 0..15 的 Byte | Java 不需要床的方块实体，不产出 |
| flower_pot / `FlowerPot` | 植物进盆栽名 | PlantBlock 不是 compound，或缺 name/states | 植物转不出或没有对应盆栽时保留空花盆 |
| banner / `Banner` | 底色进方块名 | Base 不是 0..15 的 Int | 图案 Patterns 丢弃，不产出方块实体 |
| shulker_box / `ShulkerBox` | facing | facing 不是 0..5 的 Byte | 箱内物品丢弃，不产出方块实体 |
| jukebox / `Jukebox` | has_record | 不报错 | 唱片物品丢弃，has_record 可能为 true 而没有唱片 |
| lectern / `Lectern` | has_book | hasBook 不是 Byte | 书丢弃，has_book 可能为 true 而没有书 |
| note_block / `Music` | note | note 不是 0..24 的 Byte | instrument 留给 fixer |
| skull / `Skull` | 落地头颅的 rotation | 落地头颅缺 Float Rotation | 玩家头颅的皮肤丢弃，不产出方块实体 |
| chest / `Chest` | 双箱 type=left/right；产出方块实体 | Items 不是 compound 列表；pairx/pairz 不是 Int；配对箱没有水平 facing | 转不出的物品逐条跳过；配对坐标不相邻时按单箱；LootTable 原样复制，不改路径格式 |
| sign / `Sign`、`HangingSign` | 产出方块实体 | FrontText 不是 compound | 颜色取最近的 16 种染料；每面只取前 4 行；格式码不解析 |
| item_frame / `ItemFrame`、`GlowItemFrame` | 方块变成空气，产出 Java 展示框实体 | SourceState 不是 frame / glow_frame，或 facing_direction 不是 0..5 | 物品转不出时保留空框（地图不是方块物品，目前都转不出）；Rotation 写 [0,0]，未在游戏里验证 |

只有 chest 和 sign 产出 Java 方块实体，只有 item_frame 产出 Java 实体，其余规则只改方块状态。

### bedrock.ConvertChunk：整根柱子（阶段 1）

```text
ConvertChunk(*cellerbedrock.Chunk) → (*ConvertedChunk, err)
ConvertSection(pos, dim, *cellerbedrock.Section, []blockEntity) → *ConvertedChunk
  第 0 层：按调色板条目做 ConvertState，失败的条目输出空气
  方块实体：对上表注册的 id 调用规则，产出的方块实体坐标改写为世界坐标，实体进 Entities
  第 1 层：water / flowing_water 让最终状态附带 waterlogged="true"
  err（仅 ConvertChunk，整根柱子失败）：任一 section 解码失败
        不支持的 subchunk 版本（只支持 1、8、9）、运行时调色板、
        调色板条目缺 states 或 Int version（展平前的 name+val 条目属于此类）、
        索引越界、尾部多余字节
  Diagnostics（柱子照常输出）：
    unknown_block  两步转换失败的源状态，输出空气；同一状态合并，Count 为格数，没有坐标
    nbt_error      方块实体记录整体解码失败；整根柱子跳过实体规则，方块照常
    rule_error     实体缺 Int x/y/z、不在本柱子或所在 section 不存在、缺字符串 id（这几种没有坐标）
                   或规则返回 err（有坐标和基础状态）；该实体的效果丢弃，保留基础状态
    unsupported_block_entity  没有规则的方块实体，按 BlockEntityID 合并，Count 为条数；只保留基础状态
      例如熔炉、木桶、漏斗、发射器、投掷器、酿造台的物品，刷怪笼、命令方块、信标、蜂巢的数据
  降级（没有诊断）：
    单个方块状态的所有降级，包括 179 个映射成空气的方块名
    基础状态转不出的格子不执行实体规则（已计入 unknown_block）
    第 1 层只识别水；waterlogged 会附加到任意基础状态上，包括 Java 不能含水的方块和空气
      waterlogged 是本仓库 IR 的扩展属性，不在 Java 属性表里，写出存档时由写出方决定怎么处理
    Biomes 恒为 nil；普通实体（生物、盔甲架等）不读取，Entities 里只有展示框
```

## 导出 Java 存档

`export.ConvertWorld` 扫描源世界并逐根写出 Java 柱子，返回转换报告。

```text
ConvertWorld(bedrockWorld, javaWorld, {Dimensions, Strict}) → (Report, err)
  逐根柱子：ScanChunks → ConvertChunk → ToJavaChunk / ToJavaEntities → 写 region / entities
  不跑 fixer；需要按邻居重算形状的格子登记进 PostProcessing，游戏加载区块时重算
    登记：栅栏、栅栏门、玻璃板、铁栏杆、楼梯、墙、红石线、绊线、紫颂植物
    其余方块不登记：重算会让缺支撑的方块掉落
  err：Strict 时第一根 ConvertChunk 失败的柱子；写盘失败
  Report：每个维度的柱子数、失败的柱子、各诊断种类合计、未知状态前 10、
          不支持的方块实体按 id 合计、超出 Java 高度被丢弃的 section 数、方块实体数、实体数

```

写出的 Java 区块：

```text
ToJavaChunk(ConvertedChunk, 4189) → java.Chunk
  Status=minecraft:full，yPos 为维度最低 section，isLightOn=0，Heightmaps 为空
  超出目标高度的 section 丢弃（主世界 -4..19，下界和末地 0..15）
  waterlogged 原样写出；Java 不能含水的方块上的 waterlogged，游戏加载时会丢掉
ToJavaEntities(ConvertedChunk, 4189) → {DataVersion, Position, Entities} 或 nil（没有实体）
ConvertLevel(Level, 4189) → (level.dat, LevelReport, err)
  从 Bedrock 取 LevelName、SpawnX/Y/Z、RandomSeed、GameType、Difficulty、Time、commandsAllowed
  取不到或类型不对时用 Java 默认值，并列进 LevelReport.DefaultedFields
  Time 同时写进 Time 和 DayTime；WorldGenSettings 写原版三维度生成器，种子取 RandomSeed
```

没有在游戏里验证的：空 Heightmaps 和 `isLightOn=0` 是否让游戏重算高度图和光照，PostProcessing 是否按预期重算形状，`level.dat` 能否直接打开。

## 转换源区块

源数据中的床颜色、花盆植物和第二层水，需要在源数据仍可读取时合并。
`bedrock.ConvertChunk` 接收 `*celler/bedrock.Chunk`，返回 `*converter.ConvertedChunk`。
后者保存整根柱子的 Java 状态、Java 方块实体和诊断，可以作为转换结果缓存。
方块实体是附着在某个方块上的 NBT 数据，例如箱子物品。
NBT 是 Minecraft 使用的带类型的数据格式。

基础映射按调色板条目执行。
调色板让多个格子引用同一个方块状态，避免重复存储。
转换缓存以包含版本号的 Bedrock 条目为键，映射使用条目的源状态。
Java 状态用 `ir.IRBlock` 表示，即方块名与属性的规范文本。

基础映射完成后，实体规则根据源 NBT 修改状态或产出 Java 方块实体。
床的颜色进入方块名，花盆植物进入盆栽名称。
箱子用配对箱的世界坐标确定双箱左右半，并转换箱内物品、名称和战利品表路径。
告示牌和悬挂告示牌保留正反面文字、最近的染料颜色、发光状态和封蜡状态。
旧版告示牌的顶层文字作为正面，缺失的背面使用空白文字。
规则的输入同时包含 section 内坐标和世界绝对坐标。
输出实体的 NBT 坐标与其 `Pos` 一致。
规则结构见[床](bedrock/blockentities/bed/NBT.md)和[花盆](bedrock/blockentities/flower_pot/NBT.md)。

实体规则执行后，同一格第二层的 `water` 或 `flowing_water` 会让最终状态附带 `waterlogged="true"`。
`waterlogged` 表示含水，仓库将它作为可附加到方块状态的扩展属性。
实际出现的层组合共享结果，依赖单格 NBT 的实体规则结果独立处理。

section 解码失败通过 `error` 返回。
其余可恢复的问题随区块放进 `Diagnostics`：

- `unknown_block`：未知方块输出空气，相同源状态按受影响格数合并计数。
- `nbt_error`：实体列表无法解码，区块的方块状态仍可输出。
- `rule_error`：单条实体的字段或规则出错，保留基础状态。
- `unsupported_block_entity`：方块实体没有对应规则，按 id 合并计数，只保留基础状态。

没有生物群系映射的 section，其 `Biomes` 为 nil。

## 按坐标读取与缓存

浏览或区域导出通常只需要一部分柱子。
`adapt.NewConverted` 接收一个绑定来源和维度的 Bedrock 读取回调，按坐标转换并缓存结果。
`Chunk(ctx, pos)` 返回只读共享的 `ConvertedChunk`；缺失柱子返回 `nil, nil`。
源回调用 `fs.ErrNotExist` 表达缺失，支持包装后的错误。

需要缓存其他产生 Java 转换结果的计算时，使用 `adapt.NewCachedSource`。
它接收 `ReadConvertedChunk` 回调，负责同坐标请求合并、取消与按字节预算淘汰。
外层的修复缓存也使用这个实现，因此两层拥有独立条目但共享调度语义。
`Options.CacheBytes` 为零时使用 64 MiB，负值只合并在途请求，超预算的单根结果不留存。
预算包含调色板、NBT、诊断和条目开销的估算，不等于进程内存上限。

同一来源可以并发调用，不同坐标的读取回调也可能并发执行。
每个调用者只取消自己的等待，最后一个等待者退出时才取消共享任务。
已经进入不可中断读取或转换的任务可能继续运行，但结果不会覆盖重试或进入缓存。
因此返回取消错误并不表示底层读取已经退出；句柄关闭由回调拥有者协调。
来源内容或维度改变时，应创建新实例。

返回的调色板、NBT、诊断都按只读共享，调用方修改前先复制所需部分。
需要邻接修复时，将 `Converted` 交给外层的 `neomega-converter/adapt.NewFixed`。

## 运行示例

使用 Go 1.25.5，在父工作区根目录运行：

```sh
go run ./neomega-converter-core/examples/convert-chunk
```

[完整示例](examples/convert-chunk/main.go) 在内存中构造蓝床与含水台阶，不需要已有存档。输出包含蓝床状态与带 `waterlogged="true"` 的台阶状态；可恢复的问题另从 `Diagnostics` 读取。

维护本模块的目录边界、规则扩展和验收命令见 [AGENTS.md](AGENTS.md)。
