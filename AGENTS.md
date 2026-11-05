# 维护 neomega-converter-core

本模块在 Bedrock 与 Java 之间转换方块、方块实体、物品、实体和存档：`be2je` 是 Bedrock → Java，`je2be` 是 Java → Bedrock。消费者入口和兼容性限制见 [README.md](README.md)，工作区命令与模块引用规则见 [根 AGENTS.md](../AGENTS.md)。

转换结果由根包的 `ConvertedChunk`（Java 结果）和 `BedrockChunk`（Bedrock 结果）保存。`be2je` 先转换基础调色板，再应用单格方块实体规则，最后合并第二层水；`adapt` 缓存整根转换结果，`export` 负责 Java 存档写出。

## 边界与变更约束

本模块依赖方块、NBT 与存档基础库，不依赖渲染器或 fixer。邻接修复由应用层组合。保留 `ConvertSection` 与 `ConvertChunk` 共用实现的结构，修改状态映射、实体规则、诊断或写出行为时，同时修订 README 中对应的兼容性说明。

缓存返回的数据为只读共享。修改缓存取消与淘汰逻辑时，应保持同坐标请求合并、调用者独立取消和过期任务不写回的语义。原始区块解码失败与可恢复诊断不能互相替代。

## 添加规则

先确定规则需要读取什么数据，再把它放在能够取得这些数据的位置。

| 规则依赖什么 | 例子 | 写在哪 |
|---|---|---|
| 只看方块名和属性 | 某方块映射错、新版本改名 | neomega-blocks 的 block-converts 表 |
| 调色板条目的版本号 | 旧版本状态升级 | `be2je.ConvertChunk` 的调色板条目转换 |
| 同一格的第二层 | 含水 | `be2je.ConvertChunk` 的层组合 |
| 同一格的源方块实体 | 床颜色、花盆、旗帜底色、箱子物品、告示牌文字 | `be2je/blockentities/<子包>`，并在 `be2je/convert.go` 的表里加一行 |
| 物品 | 附魔、物品变体、嵌套容器 | `be2je/items`，由方块实体规则调用 |
| 邻居的 Java 状态 | 栅栏、楼梯、snowy、树叶距离 | `neomega-viewer-core/fixer/java-postprocess` |
| 生物群系 | 数字 id 到 Java 名、Data3D/2D | `be2je.ConvertChunk` 的群系合并 |
| 方块实体要变成 Java 实体 | 展示框 | `be2je/blockentities/<子包>`，在 `Result.Entity` 里返回 |
| 普通实体 | 生物、盔甲架、掉落物 | `be2je/actors` |
| 世界元数据、存档写出 | `level.dat`、region、PostProcessing 名单 | `export/` |

新增或修改 NBT 相关的转换时，同时更新对应目录的 `NBT.md`：写清源字段、目标字段
和降级行为。规则子包的文档就在子包根目录，物品、实体、区块和 level.dat 分别在
`be2je/items/NBT.md`、`be2je/actors/NBT.md`、`be2je/NBT.md`、`export/NBT.md`。
Java → Bedrock 方向的入口与差异见 `je2be/NBT.md`。

方块实体规则通过 `blockentities.Input` 读取基础状态和源 NBT。
规则返回的 `Result` 总是包含状态，附加实体可以为 nil。
错误返回给区块入口，入口丢弃该条输出并记录诊断。
注册表由 `be2je` 持有，让规则子包可以依赖共享类型而不形成循环导入。`je2be` 的规则和注册表结构与之对称。

箱子物品由 `items.ConvertItem` 转换。
方块物品走方块表；非方块物品、旧版改名和药水/书本由生成表与专门分支处理。
附魔、耐久、名称、lore、颜色和容器内容等写入 Java 物品组件。
无法转换的条目整体跳过，所有条目都跳过时仍保留空箱实体。
范围与文字近似见 [items 包说明](be2je/items/doc.go)。

## 映射数据

物品名、生物群系、旗帜图案和实体名由 [third_party](third_party/README.md) 生成并内嵌。
不要在业务代码里手写这些表，也不要直接改 `data/tables.json.gz`。
换版本时改 `third_party/sources.lock.json` 的 ref、commit 与 sha256，再运行生成器。
方块表与旧状态升级规则仍由 neomega-blocks 维护，不要复制进本模块。

## 验收

从工作区根目录执行：

```sh
go run ./neomega-converter-core/examples/convert-chunk
./check.sh neomega-converter-core
```

新增例子应放在 `examples/<场景>/main.go`，用显式类型和中文阶段注释解释源版本、坐标及错误处理，避免要求读者查内部代码才能运行。

## 重建回归数据

存档回归测试使用 [be2je/testdata/subchunk.json](be2je/testdata/subchunk.json)。
它保存子区块 `(0, -4, 0)` 中每个非空气格子的两层源方块及 SNBT，也就是 NBT 的文本表示。
对应世界高度为 `-64..-49`。
测试将这些数据构造成 celler 区块，再调用 `ConvertChunk`。
生成程序按 version 9 的存储格式逐层读取调色板。
在联合工作区中，从本模块目录运行以下命令可重新提取 fixture。
LevelDB 在打开时可能写文件，所以先整体复制数据库：

```sh
fixture_tmp=$(mktemp -d)
cp -R ../resource/基岩版/db "$fixture_tmp/brdb"
go run ./be2je/testdata/generate.go -db "$fixture_tmp/brdb"
go test ./be2je -run TestSavedSubchunk -v
```
