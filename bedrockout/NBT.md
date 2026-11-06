# Bedrock 柱子写出

`bedrockout` 把 `converter.BedrockChunk` 写成 Bedrock 1.21.50 的数据库记录。Java 存档转换和 mcstructure 转换都经过这里，所以一根 Bedrock 柱子在数据库里长什么样，只在这一处决定。

## 区块记录

`EncodeChunk` 组装一根柱子：

- subchunk：version 9，基础层加可选的水层；
- `0x31`：方块实体列表，每项带 `id/x/y/z`；
- `Data3D`：每 section 的 16³ 群系，按维度补齐到 24（主世界）或 16（下界、末地）段，缺失的段写 plains；
- `digp`/`actorprefix`：实体；
- `0x2C`（chunk version 42）和 `0x36`（FinalizedState 2），让游戏不重新生成这根柱子。

`EncodeChunk` 只产出独占的字节，不访问数据库，因此可以在并行任务里调用。

## 批量提交

LevelDB 按批次提交更快，但批次越大占的内存越多。`Batch` 把编码好的柱子按源顺序攒到 8 MiB 再原子提交，超大的单柱单独提交。提交时数据库还要复制一份，所以调用方按两倍 `BatchBytes` 从内存预算里预留。

`NewBatch` 的 `newWorld` 表示目标坐标都还没写过，提交时跳过旧记录清理。`Add` 的回调在柱子真正提交后执行，整图转换据此只统计已经落盘的柱子。`WriteChunk` 是只写一根柱子的简便形式。

## level.dat

`NewLevelData` 只含版本字段：`StorageVersion=10` 和 `lastOpenedWithVersion=[1,21,50,0]`。调用方填写世界内容，没写的字段由游戏取默认值。`Level` 把它包成带文件头版本的 `level.dat`。
