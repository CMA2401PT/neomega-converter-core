# third_party：从上游生成并内嵌的映射数据

这里存放转换需要的**非方块**映射数据。数据由 [gen](gen/main.go) 从上游生成，
以 `data/tables.json.gz` 入库，通过根目录的 `tables` 包只读访问。
调用方不需要知道数据来自哪里、是哪个版本；需要改数据时只改这里。

方块状态与旧状态升级规则不在这里，仍由 `neomega-blocks` 维护。
把方块表也搬进来会让同一份映射出现两个真相源。

## 文件

| 路径 | 作用 |
|---|---|
| `sources.lock.json` | 上游仓库、ref/commit、许可，以及每个输入文件的 sha256 |
| `gen/main.go` | 读取锁定版本的输入，校验后生成 `data/tables.json.gz` |
| `data/tables.json.gz` | 唯一产物，入库；版本头、来源、统计和各表都在里面 |
| `tables.go` | `//go:embed` 加载器与只读访问器 |

## 表里有什么

| 表 | 形状 | 语义 |
|---|---|---|
| `items` | `(Bedrock 名, data value) → Java 物品 id` | 精确 data 查不到时回退 data 0；同一键有多个 Java 候选时优先取与 Bedrock 同名的那个 |
| `biomes` | `Bedrock 群系数字 id → Java 群系名` | Geyser 的 Java↔Bedrock 对应与 pmmp 的编号合并；同 id 多候选时优先同名 |
| `patterns` | `Bedrock 图案代码 → Java 图案名` | pmmp 的代码表；上游缺的 4 个新图案由转换规则自己补 |
| `entities` | Bedrock 实体标识集合 | 只用于判断“已知/未知实体”，不包含 Java 侧映射 |
| `java_items` / `java_biomes` / `java_patterns` | Java → Bedrock | 反向索引用 Geyser/pmmp 的原始方向生成，不由正向表反推；用于 Java → Bedrock 转换 |

物品表覆盖全部 Java 物品（含块物品）。块物品在转换时仍优先走
`neomega-blocks` 的 data value 转换，这张表负责补它不认识的名字和所有非块物品。

## 来源与版本

| 来源 | ref | commit | 许可 |
|---|---|---|---|
| [GeyserMC/mappings](https://github.com/GeyserMC/mappings) | `feature/1.21.4` | `e8089e117605e60a5ced120926c49fae7617c665` | MIT |
| [pmmp/BedrockData](https://github.com/pmmp/BedrockData) | `bedrock-1.21.50` | `6e819f36d781866ce63d2406be2ce7f2d1afd9ad` | CC0-1.0 |

表头记录 Java 1.21.4 与 Bedrock 1.21.50，和本仓库的 Java 方块表、NEMC 目标版本一致。
换版本时先改 `sources.lock.json` 的 ref/commit/sha256，再重新生成。

## 复现

在模块目录执行。先把锁定版本的输入取到临时目录：

```sh
mkdir -p /tmp/mcdata/geyser /tmp/mcdata/pmmp
for f in items.json biomes.json; do
  curl -sS -o /tmp/mcdata/geyser/$f \
    "https://raw.githubusercontent.com/GeyserMC/mappings/e8089e117605e60a5ced120926c49fae7617c665/$f"
done
for f in biome_id_map.json banner_patterns.json entity_id_map.json; do
  curl -sS -o /tmp/mcdata/pmmp/$f \
    "https://raw.githubusercontent.com/pmmp/BedrockData/6e819f36d781866ce63d2406be2ce7f2d1afd9ad/$f"
done
```

然后生成：

```sh
go run ./third_party/gen -geyser /tmp/mcdata/geyser -pmmp /tmp/mcdata/pmmp
```

生成前会按锁文件逐个核对 sha256，不匹配就退出。输出按名称排序、gzip 头固定，
同样的输入得到同样的字节；把 `data/tables.json.gz` 一起提交。

原始输入不入库：它们有各自的版本节奏，入库后容易和锁文件脱节。

## 生成时做的校验

- 每个输入文件的 sha256 必须与锁文件一致。
- 块物品的 Java 名必须能在 `neomega-blocks` 的 Java 方块表里找到。种子、红石、
  可可豆等少数物品放置的方块与物品名不同，允许 2% 误差；超过即失败。
- 统计写进表头：Java 物品数、键数、冲突数、块物品数与不匹配数、群系/图案/实体数。
  当前值是 1385 个物品、1379 个键、5 个冲突、955 个块物品、14 个不同名、60 个群系
  （2 个冲突）、38 个图案、129 个实体名；反向索引另有 1385 个 Java 物品、65 个
  Java 群系和 38 个 Java 图案名。

## 许可

GeyserMC/mappings 以 MIT 许可发布，pmmp/BedrockData 以 CC0-1.0 发布。
两者的版权与许可声明见 [NOTICE.md](NOTICE.md)。生成物只是映射数据，不包含
Minecraft 的游戏资源。
