# Java → Bedrock 的 NBT 转换总览

`je2be/` 是反方向：源是 Java NBT，目标是 Bedrock。方块状态经 `neomega-blocks` 的
`ConvertFromJava` 转成 NEMC/Bedrock 状态，物品、方块实体、实体和 `level.dat` 由
本目录的规则处理。

## 两个方向不共用类型

- `be2je/blockentities.Input` 的 Base 是 Java 状态、Source 是 Bedrock NBT；
- `je2be/blockentities.Input` 的 Base 是 Bedrock 状态、Source 是 Java NBT。

两种状态都是 `ir.IRBlock`，但属于不同 hash 空间。不要把一边的输出直接塞进另一边的
写入器；必须经过 `ConvertFromJava`（或 `ConvertFromBedrock`）。

## 逐项文档

| 转换 | Java 源字段 | 文档 |
|---|---|---|
| 方块状态 | 调色板 + `waterlogged` | [je2be 包说明](convert.go) 与 `be2je/blockentities` 的反向规则 |
| 物品 | `id`、`count`、`components` | [items](items/NBT.md) |
| 普通实体 | `id`、`Pos`、`Rotation`、`item`、`equipment` 等 | [actors](actors/NBT.md) |
| level.dat | `Data` 里的世界字段 | [export](export/NBT.md) |
| 区块记录 | subchunk、方块实体、群系、实体 | [bedrockout](../bedrockout/NBT.md) |
| 箱子 | `Items`、`CustomName`、`LootTable`、`type` | [箱子](blockentities/chest/NBT.md) |
| 熔炉 | `Items`、`lit_time_remaining` 等 | [熔炉](blockentities/furnace/NBT.md) |
| 旗帜 | `patterns`、方块颜色 | [旗帜](blockentities/banner/NBT.md) |
| 告示牌 | `front_text`/`back_text` | [告示牌](blockentities/sign/NBT.md) |
| 潜影盒 | `Items`、`facing` | [潜影盒](blockentities/shulker_box/NBT.md) |
| 唱片机 | `RecordItem` | [唱片机](blockentities/jukebox/NBT.md) |
| 讲台 | `Book`、`Page` | [讲台](blockentities/lectern/NBT.md) |
| 酿造台 | `Items`、`Fuel`、`BrewTime` | [酿造台](blockentities/brewing_stand/NBT.md) |
| 信标 | `primary_effect`、`secondary_effect` | [信标](blockentities/beacon/NBT.md) |
| 刷怪笼 | 计时字段、`SpawnData` | [刷怪笼](blockentities/mob_spawner/NBT.md) |
| 营火 | `Items`、`CookingTimes` | [营火](blockentities/campfire/NBT.md) |
| 命令方块 | `Command`、`LastOutput` | [命令方块](blockentities/command_block/NBT.md) |
| 末地门 | `exit_portal`、`Age` | [末地门](blockentities/end_gateway/NBT.md) |
| 饰纹罐 | `sherds`、`item` | [饰纹罐](blockentities/decorated_pot/NBT.md) |

每个 Java 源字段到 Bedrock 目标字段的对照都在上面链接的文档里；Bedrock → Java
的反向字段见 `be2je/blockentities/<type>/NBT.md`。

## 写出

`BedrockChunk` 由 [bedrockout](../bedrockout/NBT.md) 编码成数据库记录，`je2be/export` 把读源、转换和批量提交接成整图转换，并转换 `level.dat`。

新建目标可使用 `ConvertOptions.NewWorld` 跳过清理旧区块记录的扫描。默认模式保留替换语义。`MemoryMiB`、`Workers` 和 `Context` 控制预算、任务上限和取消；详细生命周期见模块 README。
