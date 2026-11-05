# Java 熔炉 → Bedrock Furnace

Java 的熔炉、高炉、烟熏炉分别对应 Bedrock 的 `Furnace`、`BlastFurnace`、`Smoker`。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `Items` | `Items`，经 `je2be/items` 转换并保留 `Slot` |
| `CustomName` | JSON 文本转 § |
| `LootTable` | 转成 `loot_tables/<path>.json` |
| `LootTableSeed` | 同名 Long |
| `lit_time_remaining` | `BurnTime`（Short） |
| `cooking_time_spent` | `CookTime`（Short） |
| `lit_total_time` | `BurnDuration`（Short） |

方块名决定 Bedrock 实体 id，Java 的 `cooking_total_time` 在 Bedrock 没有对应字段，丢弃。

## 降级

`Items` 结构错误返回 error；物品失败记 `unknown_item`。计时字段缺失时留空，由游戏
补默认值；字段不是整数时同样跳过。
