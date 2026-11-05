# 容器类方块实体的公共 NBT

`Barrel`、`Hopper`、`Dispenser`、`Dropper`、`EnderChest` 的 Bedrock NBT 结构相同，
Java 侧也都使用与方块同名的方块实体 ID。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `Items` | List<Compound> | 物品列表，含 `Name`、`Count`、`Slot`、可选 `tag` |
| `CustomName` | String | 自定义名称，可能带 `§` 格式码 |
| `LootTable` | String | 战利品表路径 |
| `LootTableSeed` | Long | 战利品表种子 |

## 转换

`Common` 把两个版本共有的部分组装成 Java 方块实体：

- `Items` 逐条经 `items.ConvertItemReported`，保留 `Slot`；
- `CustomName` 经 `textcomponent.Styled` 变成 JSON 文本组件；
- `LootTable` 去掉 `loot_tables/` 前缀和 `.json` 后缀，补 `minecraft:`；
- `LootTableSeed` 原样复制。

调用方传入目标 Java 方块实体 ID，因此同一套逻辑也被箱子、熔炉等复用。

## 降级

单个物品转不出时跳过并记 `unknown_item`，容器本身保留；
`Items` 不是 compound 列表时返回 error。
