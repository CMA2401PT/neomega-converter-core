# Bedrock 存档 → Java 存档

Java 的 `level.dat` 是 gzip 压缩的大端 NBT；Bedrock 使用文件头加未压缩的小端 NBT。两版的字段名、数值宽度和版本布局也不同。
`ConvertLevel` 从 `javaout.NewLevelData` 的新世界出发，只覆盖两版含义一致的字段；取不到的字段保留新世界的值，并记进 `LevelReport`。

## 并行写出与错误

整图导出将 `be2je` 转换和 `javaout` 的编码、zlib 压缩放在独立任务中，目标世界只接收已经压缩的记录，写入与报告按源扫描顺序执行。转换或编码失败的柱子记进 `FailedChunks`，写盘失败中止整图。柱子的写出格式见 [javaout](../../javaout/NBT.md)。

任务窗口受内存预算限制；严格模式只写失败柱子之前的结果。取消或错误返回前等待所有任务退出，已写入的区块仍然保留。邻接修复由应用层在转换完成后独立执行。

## 源字段到 Java 字段

| Bedrock | Java `Data` 字段 | 说明 |
|---|---|---|
| `LevelName` | `LevelName` | 缺失时 `New World` |
| `SpawnX` / `SpawnY` / `SpawnZ` | 同名 | 类型统一为 Int |
| `SpawnRotation` | `SpawnAngle` | Float |
| `RandomSeed` | `WorldGenSettings.seed` | Long |
| `GameType` | `GameType` | Int |
| `Difficulty` | `Difficulty` | Int → Byte |
| `IsHardcore` | `hardcore` | 0/1 Byte |
| `commandsAllowed` | `allowCommands` | 0/1 Byte |
| `Time` / `currentTick` | `DayTime` / `Time` | 昼夜时间与总时间 |
| `LastPlayed` | `LastPlayed` | Bedrock 是秒，Java 是毫秒 |
| `Generator`、`FlatWorldLayers` | `WorldGenSettings.dimensions` | `Generator=2` 时写 flat 生成器 |

## 游戏规则

Bedrock 把规则存在 `level.dat` 顶层（`dodaylightcycle` 等），Java 1.21.4 放在
`Data.GameRules` 里，名字是 camelCase。规则表只列两版都存在的 27 条；
Byte 值写 `"true"`/`"false"`，Int 值原样写。

## 天气

`rainLevel`/`lightningLevel` 大于 0 写 `raining`/`thundering`；
`rainTime`/`lightningTime` 对应 `rainTime`/`thunderTime`；`clearWeatherTime` 固定 0。

## 平坦世界

`FlatWorldLayers` 是 JSON，含 `biome_id` 和 `block_layers`。
每个 `block_name` 先经方块表转成 Java 名，`count` 写 `height`，
群系编号经 `third_party` 转成 Java 名。解析失败时退回噪声生成器。

## 降级

`GenerateStructures` 缺失按 1；其余缺失字段进 `DefaultedFields`。
不写玩家数据、地图、记分板、世界边界和图标。
