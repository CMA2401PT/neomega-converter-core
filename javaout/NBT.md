# Java 柱子写出

`javaout` 把 `converter.JavaChunk` 写成 Java 1.21.4（DataVersion 4189）的 region 记录。Bedrock 存档转换和 Java 结构文件转换都经过这里，所以一根 Java 柱子在存档里长什么样，只在这一处决定。

## 地形记录

`Chunk` 只补游戏加载所需的容器字段，不改变方块状态：

- `DataVersion`、`xPos/zPos/yPos`、`Status=minecraft:full`、`sections`、`block_entities`；
- 空的 `block_ticks`、`fluid_ticks`、`structures` 和 `Heightmaps`；
- `isLightOn=0`，让游戏重算光照；
- `PostProcessing`，登记需要按邻居重算形状的格子。

登记的方块是栅栏、栅栏门、玻璃板、铁栏杆、楼梯、墙、红石线、绊线和紫颂植物。其余方块不登记，因为重算会让缺支撑的方块掉落。`waterlogged` 原样写出；Java 不能含水的方块上的 `waterlogged`，游戏加载时会丢掉。

## 区块高度

调用方用 `Height` 指定写出的 section 闭区间。`DimensionHeight` 给出原版维度的范围：主世界 -4..19，下界和末地 0..15。`yPos`、`PostProcessing` 的长度与下标、方块实体过滤都使用同一范围，范围外的 section 和方块实体被丢弃。

结构转换器传入更大的范围来保存超高结构，并另外写入匹配高度的数据包和维度类型。只扩大区块范围不会改变世界定义。

## 实体记录

Java 把实体存在独立的 entities region。`Entities` 产出 `{DataVersion, Position, Entities}`，没有实体时返回 nil，调用方跳过这条记录。

## level.dat

`NewLevelData` 是一个原版新世界的 `Data`：版本字段、出生点 (0, 64, 0)、生存模式、普通难度和原版三维度生成器。调用方只覆盖自己知道的字段。`WorldGenSettings` 和 `FlatGenerator` 构造生成器设置，1.21.4 把它们放在 `Data.WorldGenSettings` 里。
