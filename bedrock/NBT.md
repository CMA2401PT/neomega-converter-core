# 区块记录的 NBT 与转换

Bedrock 的区块是 LevelDB 里带前缀键的多条记录，Java 是 MCA region 里的一个 NBT 根。
转换后的中间结果是 `converter.ConvertedChunk`，写出时再变成 Java 区块。

## Bedrock 记录

| 键后缀 | 内容 | 转换用途 |
|---|---|---|
| `0x2F <y>` | subchunk | 方块层；只支持 version 1/8/9，最多取前两层 |
| `0x31` | 方块实体 | 连续的小端 NBT 根，逐条解码 |
| `0x32` | 旧式实体 | 连续的小端 NBT 根 |
| `0x2B` | Data3D | 3D 生物群系，含 512 字节高度图前缀 |
| `0x2D` | Data2D | 2D 生物群系，每列一个编号 |
| `0x36` | FinalizedState | 生成状态；小于 2 的柱子默认跳过 |

方块实体记录缺 `id` 或 `x/y/z` 时跳过；0x31/0x32 整体解码失败记 `nbt_error`，
section 解码失败让整根柱子失败并记进 `FailedChunks`。

## 中间结果

`ConvertedChunk` 保存：每 section 的 Java 方块调色板、4×4×4 群系、Java 方块实体、
Java 实体、诊断。方块按调色板条目转换一次，第二层水合成 `waterlogged`。

## 产出的 Java 区块

写出时补 `DataVersion`、`xPos/zPos/yPos`、`Status=minecraft:full`、`sections`、
`block_entities`、空的 `block_ticks`/`fluid_ticks`、`PostProcessing`、空 `structures`
和空 `Heightmaps`，`isLightOn=0` 让游戏重算光照。
超出目标高度（主世界 -4..19，其余 0..15）的 section 丢弃并计数。

## 查看文档

- 方块实体：`bedrock/blockentities/<类型>/NBT.md`
- 物品：`bedrock/items/NBT.md`
- 普通实体：`bedrock/actors/NBT.md`
- level.dat：`export/NBT.md`
