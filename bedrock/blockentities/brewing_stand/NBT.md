# 酿造台的 NBT

Bedrock 实体 ID 是 `BrewingStand`，Java 目标 ID 是 `minecraft:brewing_stand`。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `Items` | List<Compound> | 五个槽位：0 原料、1-3 药水、4 燃料 |
| `FuelAmount` | Byte | 剩余燃料 |
| `CookTime` | 数值 | 当前酿造时间 |

## 转换

两版的槽位顺序不同，转换时重排：

| Bedrock 槽位 | Java 槽位 |
|---|---|
| 0（原料） | 3 |
| 1、2、3（药水） | 0、1、2 |
| 4（燃料） | 4 |

`FuelAmount` 写 `Fuel`（Byte），`CookTime` 写 `BrewTime`（Short）。
公共容器字段 `CustomName`、`LootTable`、`LootTableSeed` 一并转换。

## 降级

物品转不出时跳过并记 `unknown_item`；`FuelAmount` 收窄为 Byte。
