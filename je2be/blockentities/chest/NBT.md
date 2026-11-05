# Java 箱子 → Bedrock Chest

Java 箱子方块实体 id 是 `minecraft:chest` 或 `minecraft:trapped_chest`；Bedrock 统一用
`Chest`，双箱在 NBT 里存配对坐标。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `Items` | `Items`，经 `je2be/items` 转成 Bedrock 物品并保留 `Slot` |
| `CustomName` | `CustomName`，JSON 文本转 § |
| `LootTable` | 去掉 `minecraft:`，补 `loot_tables/` 前缀和 `.json` 后缀 |
| `LootTableSeed` | 同名 Long |
| 方块状态 `type=left/right` | 计算 `pairx`、`pairz`、`pairlead` |
| 方块状态 `facing` | 决定配对方向；不写进 Bedrock 实体 |

双箱按 `clockwise(facing)` 取相邻格：`left` 取正方向，`right` 取反方向。
`pairlead` 在 `left` 时为 1。单箱不写配对字段。

## 目标字段

`id=Chest`、`Items`、`CustomName`、`LootTable`、`LootTableSeed`、`Findable=0`，
双箱另有 `pairx`、`pairz`、`pairlead`。

## 降级

`Items` 不是 compound 列表返回 error；单条物品转不出时跳过并记 `unknown_item`。
`type` 是双箱但缺少 `facing` 或朝向不认识时返回 error。坐标异常、方块状态不是箱子
由区块入口处理，不在规则里补。
