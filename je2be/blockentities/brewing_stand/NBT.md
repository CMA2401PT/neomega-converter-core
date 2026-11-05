# Java 酿造台 → Bedrock BrewingStand

两版的槽位顺序不同，这是酿造台转换的主要工作。

## 源字段（Java）

| Java 槽位 | 含义 | Bedrock 槽位 |
|---|---|---|
| 0–2 | 药水 | 1–3 |
| 3 | 原料 | 0 |
| 4 | 燃料 | 4 |
| `Fuel` | Byte | `FuelAmount`（Byte） |
| `BrewTime` | Short | `CookTime`（Int） |
| `Items` | 物品列表 | `Items`，`Slot` 按上表重排 |

## 降级

`Items` 结构错误返回 error；物品失败记 `unknown_item`。槽位不在 0–4 的条目保持原
槽位，不猜测含义；`Fuel`/`BrewTime` 缺失时省略对应字段。
