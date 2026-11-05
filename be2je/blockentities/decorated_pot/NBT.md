# 饰纹罐的 NBT

Bedrock 实体 ID 是 `DecoratedPot`，Java 目标 ID 是 `minecraft:decorated_pot`。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `sherds` | List<String> | 四个面的陶片物品 ID |
| `item` | Compound | 罐内物品 |

## 转换

`sherds` 原样复制；`item` 经物品转换，去掉 `Slot` 后写 `item`。

## 降级

`item` 转不出时省略并记 `unknown_item`；`sherds` 缺失时由 Java 使用默认砖纹。
