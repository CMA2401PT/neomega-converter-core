# 熔炉、高炉和烟熏炉的 NBT

Bedrock 的实体 ID 分别是 `Furnace`、`BlastFurnace`、`Smoker`；
Java 的目标 ID 由方块名决定，也是这三个。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `BurnTime` | 数值 | 当前燃料剩余燃烧时间 |
| `CookTime` | 数值 | 当前物品已烹饪时间 |
| `BurnDuration` | 数值 | 本次燃料总燃烧时间 |

内容物与自定义名称使用公共容器字段 `Items`、`CustomName`、`LootTable`、`LootTableSeed`。

## 转换

| Java 字段 | 来源 | 类型 |
|---|---|---|
| `lit_time_remaining` | `BurnTime` | Short |
| `cooking_time_spent` | `CookTime` | Short |
| `lit_total_time` | `BurnDuration` | Short |
| `cooking_total_time` | 固定 200 | Short |

Java 1.21.4 这四个字段都是 Short；Bedrock 可能用不同宽度，读取时统一收窄。

## 降级

缺字段按 0 写出；`Items` 不是列表返回 error；物品逐条跳过并记 `unknown_item`。
