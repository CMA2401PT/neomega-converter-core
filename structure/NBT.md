# 结构文件到原生存档

结构格式通常按一个整体排列方块，存档则按区块组织。转换器从 NBT 解压流接收字段，把大数组压缩暂存，待尺寸和调色板齐全后按空间片区重排，逐片写成原生世界。这样可以处理“数组先于元数据”的合法字段顺序，也不需要完整体积的内存数组。

Java 来源保存 Java 状态；基岩来源保存基岩状态。Java 输出为 1.21.4 的区块布局，基岩输出为原生 LevelDB。文件格式解析属于 converter-core，存档与区块读写复用 celler。

## 方块数据

| 格式 | 来源字段与排列 | 写出行为 |
|---|---|---|
| Sponge schem v1/v2 | Width/Height/Length、Palette、BlockData；索引为 varint，X 最快，其次 Z、Y | Java palette；v1 取 TileEntities，v2 取 BlockEntities |
| Sponge schem v3 | Schematic 下的尺寸；Blocks.Palette、Blocks.Data、Blocks.BlockEntities | 与 v2 共用索引处理，支持多字节 varint 跨读取缓冲 |
| Alpha schematic | Blocks、Data、可选 AddBlocks；X 最快，其次 Z、Y | 合并三个数值流；AddBlocks 的低／高半字节对应偶／奇下标；ID/meta 直接查 Java 映射，未知组合报错 |
| litematic v4–v7 | Regions 下的 Position、Size、BlockStatePalette、BlockStates | 索引宽度至少 2 bit；支持跨 long 打包及按完整 long 填充的排列 |
| Java nbt 模板 | size、palette 或 palettes、blocks 中的 pos/state/nbt | 多套 palettes 选择第一套；blocks 用可复用接收器逐条暂存，不构造完整列表 |
| mcstructure v1 | size、structure.block_indices、palette.default.block_palette | X-Y-Z（Z 最快）；保留最多两层，-1 表示空位；条目 name/states/version 原样保留 |

空数组、数组长度与体积不符、越界索引、损坏或截断的 NBT 返回错误。gzip 输入在根 NBT 结束后继续校验尾部，拒绝校验失败与多余数据。未使用的元数据通过丢弃接收器消费，不创建动态数组。需要保存的调色板及实体 NBT 仍占用与其数量相关的内存。

## 坐标和高度

schem、schematic 和原版 nbt 的最小角放到原点；WorldEdit 的 Offset 不作为放置坐标。litematic 的 Position 是区域相对原点，Size 为负时沿该轴反向延伸，最小角为 Position + Size + 1。区域按名称排序，后一个区域覆盖之前的方块（包含空气）并移除被覆盖位置的旧方块实体。

mcstructure 使用 structure_world_origin 放置。Java 与 Bedrock 都会整体平移超出默认主世界范围的结构，使其落入可保存的高度；各部分保持相对位置。Java 高于 384 格时以 Y=-64 为底，写入扩展主世界高度的数据包，最高支持 2096 格。Bedrock 高于 384 格时报错。

没有源群系的 Java section 使用 plains，生成器使用空超平坦。出生点取水平中心与最高位置上方两格，viewer 会再加自己的视点高度。

## 方块实体和普通实体

Java 方块实体保留源 payload，重写 id 和绝对 x/y/z。Sponge v2 的额外字段与 v3 的 Data 子字段均保留。部分 Alpha schematic 使用 Sponge 的 Pos/Id 包装，也按该包装提取。litematic 的方块实体坐标相对区域最小角，普通实体 Pos 相对 Position。原版模板从条目的 nbt 读取实体内容，坐标取外层 pos。

Java 普通实体写入 entities region。源数据缺少实体 id 或三项 Pos 时跳过。历史实体名称、物品结构、告示牌字段、命令与其他版本专属 NBT 保持原文，不执行完整的 DataFixer 升级；因此结构能被读取和预览并不保证所有历史游戏行为已迁移。

mcstructure 的 block_position_data 通过线性下标恢复世界位置并保留方块实体 NBT。普通实体的 Pos 加上结构原点，写入 digp/actorprefix，并分配新的 UniqueID。原有实体引用关系不重新连接。两层方块不会为存盘而转换为 Java 的 waterlogged；预览时才复用基岩到 Java 的转换与修复。

## mcworld 和输出目录

mcworld 本身就是原生世界的 zip 包，转换只负责解压。支持 level.dat 位于压缩包根目录或统一的前缀目录；多个世界报错。条目路径必须位于输出目录内。转换器逐条解压到磁盘，不把整个 LevelDB 文件装入内存。

目标目录必须不存在或为空。转换在同一父目录内建立暂存目录，完成后改名发布；失败或 context 取消时清理暂存目录。已有非空世界不会被替换。viewer 可以直接复用源文件旁已存在的同名存档，这项复用策略属于应用层。

## 运行与验证

在工作区根目录执行：

```sh
go run ./apps/neomega-converter/cmd/structure-to-world -input /path/to/build.schem
./check.sh neomega-nbt neomega-blocks neomega-celler neomega-converter-core apps/neomega-converter apps/neomega-viewer
```

性能测量先构建程序，把输入和输出都放在 SSD；输出必须选新目录，以免测到拒绝覆盖的路径：

```sh
go build -o /tmp/structure-to-world ./apps/neomega-converter/cmd/structure-to-world
/usr/bin/time -l /tmp/structure-to-world -input /ssd/build.schem -output /ssd/build-world
```

Sponge 字段定义参照 [v2 规范](https://github.com/SpongePowered/Schematic-Specification/blob/master/versions/schematic-2.md)与 [v3 规范](https://github.com/SpongePowered/Schematic-Specification/blob/master/versions/schematic-3.md)。扩展高度使用 Java 1.21.4 的 dimension_type 数据结构和 pack_format 61。
