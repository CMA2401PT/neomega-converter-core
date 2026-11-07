# neomega-converter-core

本模块在 Bedrock 与 Java 存档之间互相转换，也把结构文件写成可以直接打开的原生世界。
`bedrock-to-java`、`java-to-bedrock` 和 `structure-to-world` 命令行由 [neomega-converter](../apps/neomega-converter/README.md) 组合。
一根区块柱子的水平范围是 16×16，内部按高度分成 16×16×16 的 section。
本模块依赖方块、NBT 与存档基础库，不依赖渲染器或 fixer；邻接属性修复由应用层组合。

## 整体结构

格式虽多，每一种转换都是同样的三段：读源，得到目标版本的一根柱子，再写成目标的原生记录。
中间那根柱子叫中间结果，只有两种：`JavaChunk` 装 Java 方块状态，`BedrockChunk` 装 Bedrock 方块状态。
两者定义在根包，方块状态属于不同的 hash 空间，不能互相直接写回。

```text
Bedrock 存档 ───── be2je ───────┐
Java 结构文件 ──── structure ───┴─▶ JavaChunk ───── javaout ────▶ Java 存档

Java 存档 ──────── je2be ───────┐
mcstructure ────── structure ───┴─▶ BedrockChunk ── bedrockout ─▶ Bedrock 存档

mcworld ────────── structure ─────────── 原样解压 ──────────────▶ Bedrock 存档
```

图里的 Java 结构文件指 `.schem`、`.schematic`、`.litematic` 和原版 `.nbt`。
每个包只负责图中的一段：

| 包 | 负责 | 不碰 |
|---|---|---|
| `be2je`、`je2be` | 跨版本翻译方块、方块实体、物品、实体和群系，输入输出都是内存中的柱子 | 文件、并发 |
| `javaout`、`bedrockout` | 把中间结果写成原生记录，定义区块字段、高度、记录版本和 `level.dat` 的新世界内容 | 数据从哪来 |
| `be2je/export`、`je2be/export` | 整图转换：扫描源存档、并行翻译和编码、按序写出、汇总报告，并翻译 `level.dat` | 单格规则、记录格式 |
| `structure` | 流式读取结构文件，按片区重排后构造同版本的中间结果 | 跨版本翻译 |
| `be2je/adapt` | 按坐标读取 Bedrock 柱子，翻译成 `JavaChunk` 并缓存，供浏览器和修复使用 | 写出 |

`internal/pipeline` 是整图转换和结构转换共用的有界并行调度：一个协程读源，多个任务计算，一个协程按源顺序写出。

### 新增或修改一种转换时

先判断改动落在三段中的哪一段，位置就只有一个：

- 某种方块、物品或实体翻译得不对：改 `be2je` 或 `je2be`，具体子包见 [AGENTS.md](AGENTS.md#添加规则)。
- 写出的柱子游戏读不对，例如高度、PostProcessing、记录版本或 `level.dat` 默认值：改 `javaout` 或 `bedrockout`，所有来源同时受益。
- 支持一种新的结构格式：在 `structure` 写一个流式读取器，在 `formats` 表登记它写成哪个版本，写出直接复用该版本已有的路径。
- 新的来源或目标组合，例如把 Bedrock 存档写成 Java 结构文件：用 celler 读源；版本不同时调用 `be2je` 或 `je2be` 得到中间结果；目标是存档时用对应的 `*out` 包写出，目标是新容器时为它写一个新的写出包。并行与内存预算复用 `internal/pipeline`。

## 能力一览

本模块对外提供几层相互独立的能力，调用方按需要组合。
下面用伪代码列出每层的签名、支持到哪一步、什么时候报错，以及不报错但结果降级的情况。
「降级」指调用成功，但输出比源数据少了信息或只是近似。
所列行为以当前代码为准，修改实现时请同步这一节。

```text
能力                                          输入 → 输出
# 跨版本翻译：只处理内存中的数据
be2je.ConvertState(state)                   单个 Bedrock 状态 → Java 状态, ok
items.ConvertItem(item)                     Bedrock 物品 compound → Java 物品 compound 或 nil
be2je.ConvertBlockEntity(id, Input)         单格方块实体 → Java 状态 + 可选方块实体 / 实体, ok, err
be2je.ConvertSection(pos, dim, ...)         单个 Bedrock section + 方块实体 → JavaChunk
be2je.ConvertChunk(chunk)                   Bedrock 整根柱子 → JavaChunk, err
be2je.ConvertChunkWithActors(chunk, actors) 同上并追加 digp 实体 → JavaChunk, err
je2be.ConvertChunk(pos, dim, chunk)         Java 整根柱子 → BedrockChunk, err
je2be.ConvertChunkWithActors(pos, dim, chunk, actors)  同上并追加实体

# 写成原生记录：不关心数据来源
javaout.Encode(chunk, height)               JavaChunk → 压缩好的地形与实体 region 记录
javaout.Chunk / Entities                    JavaChunk → Java 区块 / Java 实体文档
javaout.NewLevelData()                      Java 新世界的 level.dat 内容
bedrockout.EncodeChunk(chunk)               BedrockChunk → Bedrock 数据库记录
bedrockout.NewBatch(world, newWorld)        按 8 MiB 批次提交编码好的柱子
bedrockout.NewLevelData()                   Bedrock level.dat 的版本字段

# 整图转换
be2je/export.ConvertLevel(level)            Bedrock level.dat → Java level.dat
be2je/export.ConvertWorld(src, dst, opts)   Bedrock 存档 → Java 存档，不跑 fixer
je2be/export.ConvertLevel(document)         Java level.dat → Bedrock level.dat
je2be/export.ConvertWorld(src, dst, opts)   Java 存档 → Bedrock 存档
structure.ConvertFile(ctx, input, output, options...)  结构文件 → 原生 Java／Bedrock 存档目录；拒绝覆盖非空目录

# 按坐标读取
be2je/adapt.NewConverted(read, opts)        按坐标读取 Bedrock 柱子 → 缓存的 JavaChunk
```

两个方向不共用方块实体 Input。Java → Bedrock 的字段差异和逐项文档见
[je2be/NBT.md](je2be/NBT.md)。

`ConvertState` 内部使用 `neomega-blocks/presets/block-converts/v1` 的两个函数。
Bedrock → Java 的三个粒度共用同一套实现：`ConvertSection` 和 `ConvertChunk`
只差在输入是已解码的单个 section 还是整根柱子的原始记录。

### 单个方块状态

存档里的状态可能来自任意 Bedrock 版本，而 Java 转出表只认目标版本的状态。
所以转换分两步：`ConvertFromBedrock` 先把旧方块名、旧属性和缺失的属性对齐到目标版本，`ConvertToJava` 再转成 Java。
`be2je.ConvertState` 就是这两步，本模块的区块转换、花盆规则和物品转换都经过它。

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
        非方块物品：按 (Name, 顶层 Damage) 查 third_party 的物品表，缺失时回退 data 0
        旧名兼容：scute → turtle_scute、record_* → music_disc_*
        potion / splash_potion / lingering_potion、药水箭（arrow 且 Damage>0）
        written_book（标题、作者、代数、书页）、writable_book（书页原文）
        组件：
          tag.ench → minecraft:enchantments；附魔书用 minecraft:stored_enchantments
          tag.Damage → minecraft:damage，只给可损耗的非方块物品
          tag.display.Name → minecraft:custom_name（JSON 文本组件）
          tag.display.Lore → minecraft:lore
          tag.customColor → minecraft:dyed_color {rgb}
          tag.Unbreakable → minecraft:unbreakable
          tag.RepairCost → minecraft:repair_cost
          tag.Items → minecraft:container，递归转换，每项为 {slot, item}
        保留 Slot
  返回 nil，调用方跳过整个物品：
        名称为空或 air、Count<=0
        物品表查不到的名字
        药水编号越界或为 decay、附魔编号越界或字段类型错误、书页不是字符串
      以上失败都会通过 ReportItem 上报，区块诊断记 unknown_item
  降级：
        没有 Block compound 的旧物品，名称已不在目标版本时（如 wool + Damage 14）
          legacy data value 查不到，退回默认状态（white_wool）
        床、旗帜、头颅的 Damage 是颜色或种类，不是方块数据，退回默认外观
        嵌套容器里转不出的条目逐条跳过，容器本身保留
        § 格式码解析成 Java 文本组件的颜色与格式（k/l/m/n/o/r）
        dyed_color 只写 rgb，不写 show_in_tooltip
```

物品名表与生物群系表由 [third_party](third_party/README.md) 从上游生成并内嵌。
调用方只需要调用这些入口，不需要知道表的来源与版本；换版本时只改 `third_party`。

### 方块实体规则：单格

每个规则子包在 `be2je/blockentities/` 下，导出 `ID`（Bedrock 实体 id）和 `Convert`。
`be2je.ConvertBlockEntity` 按 id 从注册表选规则。

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
| banner / `Banner` | 底色进方块名、图案进方块实体 | Base 不是 0..15 的 Int；Patterns 不是 compound 列表 | 未知图案代码逐条跳过；上游缺的 4 个图案代码由规则补齐 |
| shulker_box / `ShulkerBox` | facing、箱内物品（产出方块实体） | facing 不是 0..5 的 Byte | 物品逐条跳过并记 `unknown_item` |
| jukebox / `Jukebox` | has_record、RecordItem（产出方块实体） | 不报错 | 唱片转不出时只有 has_record |
| lectern / `Lectern` | has_book、Book/Page（产出方块实体） | hasBook 不是 Byte | 书转不出时只有 has_book |
| note_block / `Music` | note | note 不是 0..24 的 Byte | instrument 留给 fixer |
| skull / `Skull` | 落地头颅的 rotation | 落地头颅缺 Float Rotation | 玩家头颅的皮肤丢弃，不产出方块实体 |
| chest / `Chest` | 双箱 type=left/right；产出方块实体 | Items 不是 compound 列表；pairx/pairz 不是 Int；配对箱没有水平 facing | 转不出的物品逐条跳过并记 `unknown_item`；配对坐标不相邻时按单箱；LootTable 改写为 Java 资源位置 |
| container / `Barrel`、`Hopper`、`Dispenser`、`Dropper`、`EnderChest` | 产出方块实体：Items、CustomName、LootTable/Seed | Items 不是 compound 列表 | 物品逐条跳过并记 `unknown_item` |
| furnace / `Furnace`、`BlastFurnace`、`Smoker` | 燃烧/烹饪计时 + 容器字段 | Items 不是 compound 列表 | 计时按 Java 的 Short 写出 |
| brewing_stand / `BrewingStand` | 槽位重排、Fuel、BrewTime + 容器字段 | Items 不是 compound 列表 | 槽位 0/1/3/4 映射为 3/0/2/4；Fuel 按 Byte 写出 |
| beacon / `Beacon` | primary/secondary effect | 不报错 | 未知效果编号省略 |
| mob_spawner / `MobSpawner` | 七个计时字段、SpawnData | 不报错 | SpawnPotentials 恒为空 |
| campfire / `Campfire` | 四个槽位、CookingTimes | 不报错 | CookingTotalTimes 恒为 600 |
| command_block / `CommandBlock` | 命令与状态字段 | 不报错 | 命令语言不翻译；LastOutput 保留原文 |
| end_gateway / `EndGateway` | exit_portal、Age | 不报错 | 坐标不是三个 Int 时省略 exit_portal |
| decorated_pot / `DecoratedPot` | sherds、item | 不报错 | 物品转不出时省略 item |
| sign / `Sign`、`HangingSign` | 产出方块实体 | FrontText 不是 compound | 颜色取最近的 16 种染料；每面只取前 4 行；§ 格式码解析成样式 |
| item_frame / `ItemFrame`、`GlowItemFrame` | 方块变成空气，产出 Java 展示框实体 | SourceState 不是 frame / glow_frame，或 facing_direction 不是 0..5 | 物品转不出时保留空框（地图不是方块物品，目前都转不出）；Rotation 写 [0,0]，未在游戏里验证 |

chest、sign、banner 和容器类规则产出 Java 方块实体，只有 item_frame 产出 Java 实体，其余规则只改方块状态。

### be2je.ConvertChunk：整根柱子（阶段 1）

```text
ConvertChunk(*cellerbedrock.Chunk) → (*JavaChunk, err)
ConvertSection(pos, dim, *cellerbedrock.Section, []blockEntity) → *JavaChunk
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
      例如蜂巢、拼图方块、结构方块、试炼刷怪笼、宝库的数据
    unknown_entity  没有 Java 对应的实体标识，按 EntityID 合并，Count 为条数
    unknown_item    物品名或物品数据转不出，按 ItemID 合并，Count 为条数
  降级（没有诊断）：
    单个方块状态的所有降级，包括 179 个映射成空气的方块名
    基础状态转不出的格子不执行实体规则（已计入 unknown_block）
    第 1 层只识别水；waterlogged 会附加到任意基础状态上，包括 Java 不能含水的方块和空气
      waterlogged 是本仓库 IR 的扩展属性，不在 Java 属性表里，写出存档时由写出方决定怎么处理
    生物群系按 Data3D 优先、2D 回退映射，未知编号和缺失 section 用 plains
    普通实体读取 0x32 记录和调用方给的 digp 实体，转换通用字段与掉落物/船等特例；
    村民交易、乘骑、宠物主人、盔甲架姿势等类型专属数据不保留
```

## Bedrock 存档转成 Java 存档

`be2je/export.ConvertWorld` 扫描源世界，将独占的区块记录交给并行任务完成 `be2je` 转换和 `javaout` 编码、压缩，再由调用协程按源顺序写出并合并报告。源与目标句柄分别由一个协程使用。严格模式按源顺序停止，后面的任务即使先完成，也不会越过失败柱子写出。

`be2je/export.ConvertOptions` 和反向的 `je2be/export.ConvertOptions` 都支持 `MemoryMiB`、`Workers` 和 `Context`。内存预算零值为 512 MiB，至少 64 MiB；并发零值按 CPU 自动选择，最多 8 个。每个存档转换任务按 64 MiB 工作区估算并发，至少执行一个任务，因此预算不是进程 RSS 硬上限。任务窗口最多保留与实际并发数相同的未交付输入和结果，另有生产者正在读取的一根柱子；慢任务不会造成无界积压。`Context` 为 nil 时使用 Background，返回前等待所有读取和计算退出。

```text
ConvertWorld(bedrockWorld, javaWorld, {Dimensions, Strict, IncludeUnfinished}) → (Report, err)
  逐根柱子：ScanChunks → 跳过未完成柱子 → be2je.ConvertChunk → javaout.Encode → 写 region / entities
  默认跳过 0x36 FinalizedState < 2 的柱子；记录缺失按已完成处理，和 mcconv 一致
  读取 digp/actorprefix 失败只记 nbt_error 并继续写出该柱地形
  柱子转换、编码或压缩失败记 FailedChunks；写盘失败中止整图
  不跑 fixer；需要按邻居重算形状的格子由 javaout 登记进 PostProcessing，游戏加载区块时重算
  err：Strict 时第一根失败的柱子；写盘失败
  Report：每个维度的柱子数、失败的柱子、诊断汇总、跳过的柱子数、
          超出 Java 高度被丢弃的 section 数、方块实体数、实体数

```

柱子的写出格式见 [javaout/NBT.md](javaout/NBT.md)：高度范围、加载字段、PostProcessing 名单和实体记录都在那里定义。

`Report` 与反向的 `je2be/export.Report` 都用 `converter.FailedChunk` 列失败柱子，用 `converter.DiagnosticSummary` 汇总诊断：按种类合计，再按未知状态、未知物品、不支持的方块实体名称细分。`converter.TopCounts` 取其中影响最大的几项，补映射时从这里开始。两个方向的报告共用同一种诊断格式，往返转换后可以直接对照。

```text
ConvertLevel(Level) → (level.dat, LevelReport)
  从 javaout.NewLevelData 的新世界出发，覆盖 Bedrock 里有的字段：
    LevelName、SpawnX/Y/Z/SpawnRotation、RandomSeed、GameType、Difficulty、
    IsHardcore、commandsAllowed、Time/currentTick、LastPlayed（秒转毫秒）、
    rainLevel/lightningLevel/rainTime/lightningTime、27 条两版同名的游戏规则、
    GenerateStructures
  取不到或类型不对时保留新世界的值，并列进 LevelReport.DefaultedFields
  Time 写总时间、DayTime 写昼夜时间；WorldGenSettings 写原版三维度生成器，种子取 RandomSeed
  Generator=2 且 FlatWorldLayers 可解析时，主世界写成 minecraft:flat 生成器，
    层高与群系按源数据转换；解析失败退回噪声生成器
```

没有在游戏里验证的：空 Heightmaps 和 `isLightOn=0` 是否让游戏重算高度图和光照，PostProcessing 是否按预期重算形状，`level.dat` 能否直接打开。

## 结构文件写成原生存档

`structure.ConvertFile(ctx, input, output)` 将没有区块索引的结构转成可以随机读取的世界。它直接从解压流接收 NBT 数组，把连续相同值合并为游程，再按小片区重排并写出区块。游程就是“值及其连续重复次数”。整个过程不创建与结构体积等大的数组，也不同时保留全图区块。

| 输入 | 输出 | 数据处理 |
|---|---|---|
| `.schem` | Java | Sponge v1／v2／v3 的调色板与变长索引 |
| `.schematic` | Java | Alpha 数字 ID、Data、可选 AddBlocks，直接查 Java 映射 |
| `.litematic` | Java | v4–v7，多区域、负尺寸和跨 long 打包索引 |
| `.nbt` | Java | 原版结构模板的 palette／palettes、blocks、entities |
| `.mcstructure` | Bedrock | 保留双层方块、条目版本、方块实体与普通实体 |
| `.mcworld` | Bedrock | 逐条解压原生世界包，不翻译其中的数据库 |

`output` 为空时，存档放在源文件旁去掉最后一个扩展名的同名文件夹，例如 `城堡.schem` → `城堡/`。非空目标目录返回错误。转换在相邻临时目录完成，成功后才发布目标目录；失败或取消会清理暂存内容。

数组可能出现在尺寸和调色板之前，所以解析时先用固定缓冲接收并压缩暂存，元数据齐全后再消费。空间重排的总缓冲为 8 MiB。写出阶段按 128×128 的水平片区并行组装、编码和压缩，每个任务最多展开 64 根区块柱：Java 结构展开成 `JavaChunk` 交给 `javaout`，mcstructure 展开成 `BedrockChunk` 交给 `bedrockout`。编码好的记录按片区顺序交给同一个写入者。片区内部保持源记录顺序，因此多区域覆盖结果不受任务完成顺序影响。

可选的 `structure.Options{MemoryMiB: 512, Workers: 0}` 控制工作预算。预算先为片区工作区预留空间，余量足够时对已经写完的压缩暂存文件建立只读 mmap，即由操作系统按需映射文件页面，解压直接读取这些页面。Darwin、Linux 和 Windows 支持映射；预算不足或其他平台使用按偏移读取。映射大小从并发预算中扣除，再按片区高度限制任务数。`MemoryMiB` 的零值为 512，最小值为 64；`Workers` 的零值按 CPU 数自动选择，最多 8 个，显式值仍受预算限制。mcstructure 共用暂存读取和有界调度，双层片区各预留 64 MiB 工作区，并为 8 MiB 数据库批次及合并副本预留 16 MiB。基岩区块与实体共同批量提交；mcworld 只逐条解压。

预算不是进程 RSS 硬上限，内存还包含调色板、实体 NBT、索引和 Go 运行时。`Options.Progress` 由调用协程串行通知，取消时等待任务退出后才释放映射和临时文件。临时数据与输出位于同一磁盘，性能测试应把源文件和输出都放在 SSD。

Java 输出使用 DataVersion 4189、空超平坦主世界和 plains 群系；普通实体写入独立 entities region。高度超出默认范围时整体平移。结构高于 384 格时附带 Java 1.21.4 的扩展高度数据包，支持最多 2096 格。Bedrock 保持原生主世界高度，高于 384 格时报错。位置、区域覆盖顺序及 NBT 保留范围见 [structure/NBT.md](structure/NBT.md)。

这是结构数据落盘能力，不是 Java 历史 NBT 的完整版本升级器。方块实体和物品的历史字段保留原样，游戏中的旧实体兼容性与扩展高度世界仍需实际验证。浏览器复用本入口生成的世界；邻接修复由浏览应用按需组合，不写回结构源文件。

## 转换源区块

源数据中的床颜色、花盆植物和第二层水，需要在源数据仍可读取时合并。
`be2je.ConvertChunk` 接收 `*celler/be2je.Chunk`，返回 `*converter.JavaChunk`。
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
每个规则子包都附一份 `NBT.md`，说明源字段、目标字段和降级行为，例如
[床](be2je/blockentities/bed/NBT.md)和[花盆](be2je/blockentities/flower_pot/NBT.md)。
区块记录的整体结构见 [be2je/NBT.md](be2je/NBT.md)；物品、普通实体和 level.dat
分别见 [items/NBT.md](be2je/items/NBT.md)、[actors/NBT.md](be2je/actors/NBT.md)、
[be2je/export/NBT.md](be2je/export/NBT.md)。

实体规则执行后，同一格第二层的 `water` 或 `flowing_water` 会让最终状态附带 `waterlogged="true"`。
`waterlogged` 表示含水，仓库将它作为可附加到方块状态的扩展属性。
实际出现的层组合共享结果，依赖单格 NBT 的实体规则结果独立处理。

section 解码失败通过 `error` 返回。
其余可恢复的问题随区块放进 `Diagnostics`：

- `unknown_block`：未知方块输出空气，相同源状态按受影响格数合并计数。
- `nbt_error`：实体列表无法解码，区块的方块状态仍可输出。
- `rule_error`：单条实体的字段或规则出错，保留基础状态。
- `unsupported_block_entity`：方块实体没有对应规则，按 id 合并计数，只保留基础状态。
- `unknown_entity`：实体标识没有 Java 对应，按 id 合并计数并跳过。
- `unknown_item`：物品名或物品字段转不出，按物品名合并计数；容器保留，只丢该条目。

没有群系记录的 section 全部写 `minecraft:plains`；源数据优先用 Data3D，其次 Data2D，
未知编号也回退 plains，避免写出的 Java section 缺少必需的 `biomes` 字段。

## 普通实体

`be2je.ConvertChunk` 读取柱子自带的 0x32 记录；`be2je.ConvertChunkWithActors` 再追加
调用方从 `digp`/`actorprefix` 读到的实体（`be2je/export.ConvertWorld` 已经这样做）。
两类记录都交给 `be2je/actors`，它只转换两版含义一致的字段：

- `id`：Bedrock 标识经一张小的改名表转成 Java id（僵尸猪灵、卫道士、村民变种等）；
- `Pos`、`Rotation`、`Motion`、`UUID`、`CustomName`、`Tags`、`Fire`、`Air`；
- `Persistent`、`IsBaby`、`Health` 和 `Armor`/`Mainhand`/`Offhand` 装备；
- 掉落物（`item`）和船（`boat`、`chest_boat` 按 `Variant` 决定木种与筏）。

瞬时实体（箭、火球、经验瓶等）直接跳过；未知标识记 `unknown_entity`。
村民交易、乘骑、宠物主人、盔甲架姿势、生物群系变种等类型专属数据不转换。

## 按坐标读取与缓存

浏览或区域导出通常只需要一部分柱子。
`be2je/adapt.NewConverted` 接收一个绑定来源和维度的 Bedrock 读取回调，按坐标转换并缓存结果。
第三个可选参数 `ReadActors` 读取 digp/actorprefix 中与地形分开的普通实体；读取失败记入 `nbt_error`，地形继续转换。
`Chunk(ctx, pos)` 返回只读共享的 `JavaChunk`；缺失柱子返回 `nil, nil`。
源回调用 `fs.ErrNotExist` 表达缺失，支持包装后的错误。

需要缓存其他产生 Java 转换结果的计算时，使用 `adapt.NewCachedSource`。
它接收 `ReadJavaChunk` 回调，负责同坐标请求合并、取消与按字节预算淘汰。
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


## Java 存档转成 Bedrock 存档

`je2be.ConvertChunk` / `je2be.ConvertChunkWithActors` 把 Java 柱子转成
`converter.BedrockChunk`，`bedrockout` 把它编码成 Bedrock 区块与实体记录，`je2be/export` 把两者接成整图转换并翻译 `level.dat`。
源 Java 句柄只读取独占的压缩记录，解压、NBT 解析、转换和 Bedrock 编码由任务并行完成。写入者按源顺序把编码记录交给 `bedrockout.Batch`，报告只计入已提交的柱子。严格模式会提交失败柱子之前的待写结果；取消时丢弃未提交批次，已写出的内容仍然保留。

`ConvertOptions.NewWorld` 默认为 false，保留已有区块的清理语义；明确新建世界时设为 true 可省去旧记录扫描。命令行只写新目标，因此会启用该选项。

总览见 [je2be/NBT.md](je2be/NBT.md)，记录格式见 [bedrockout/NBT.md](bedrockout/NBT.md)，命令行见
[apps/neomega-converter](../apps/neomega-converter/README.md)。

### 展示框的预览物品目录

`be2je.ConvertChunkWithOptions` 的 `ConvertOptions.FrameItemResolver` 和 `adapt.Options.FrameItemResolver` 可显式接入调用方的物品 ID 目录，保留原有组件校验和失败诊断。nil 保持默认行为；回调只读访问源物品，并须支持并发调用。此选项仅用于展示框，默认整图转换输出版本保持原有约定。
