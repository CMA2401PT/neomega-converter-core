# Java 存档 → Bedrock 存档

`ConvertWorld` 串行扫描 Java region，在并行任务里解码、经 `je2be` 转换、经 `bedrockout` 编码，再按源顺序交给 `bedrockout.Batch` 提交。读记录、转换或编码失败的柱子记进 `FailedChunks`，写库失败中止整图。

## level.dat

`ConvertLevel` 把 Java `Data` 写回 Bedrock 顶层字段：名称、出生点、种子、游戏模式、
难度、hardcore、命令、时间、天气、游戏规则、平坦世界和 `GenerateStructures`。
版本字段来自 `bedrockout.NewLevelData`。

缺失字段记进 `LevelReport.DefaultedFields`；没有对应 Bedrock 字段的 Java 数据直接丢弃。
