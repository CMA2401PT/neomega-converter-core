# Java → Bedrock 的区块与 level.dat 写出

## 区块

`WriteChunk` 组装 Bedrock 柱子：

- subchunk：version 9，基础层 + 可选水层；
- `0x31`：方块实体列表（每个带 `id/x/y/z`）；
- `Data3D`：每 section 的 16³ 群系，按维度补齐到 24/16 段；
- `digp`/`actorprefix`：实体；
- `0x2C`（chunk version 42）和 `0x36`（FinalizedState 2）。

## level.dat

`ConvertLevel` 把 Java `Data` 写回 Bedrock 顶层字段：名称、出生点、种子、游戏模式、
难度、hardcore、命令、时间、天气、游戏规则、平坦世界和 `GenerateStructures`。
写入 `StorageVersion=10`、`lastOpenedWithVersion=[1,21,50,0]`。

缺失字段记进 `LevelReport.Defaulted`；没有对应 Bedrock 字段的 Java 数据直接丢弃。
