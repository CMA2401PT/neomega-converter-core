# 营火的 NBT

Bedrock 实体 ID 是 `Campfire`，灵魂营火由方块名区分；Java 目标 ID 是
`minecraft:campfire` 或 `minecraft:soul_campfire`。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `Item1`..`Item4` | Compound | 四个烹饪槽里的物品 |
| `ItemTime1`..`ItemTime4` | 数值 | 各槽已烹饪时间 |

## 转换

物品经 `items.ConvertItemReported` 转换，补 `Slot = i-1`；
`ItemTime1..4` 组成 Java 的 `CookingTimes` 整数数组；
`CookingTotalTimes` 固定写四个 600。

## 降级

物品转不出时跳过并记 `unknown_item`；缺槽位写空余位，不报错。
