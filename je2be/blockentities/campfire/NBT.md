# Java 营火 → Bedrock Campfire

Java 用带 `Slot` 的物品列表和 `CookingTimes` 数组；Bedrock 用四组扁平字段。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `Items` 中 `Slot=0..3` | `Item1`..`Item4`，删掉 `Slot` |
| `CookingTimes[0..3]` | `ItemTime1`..`ItemTime4`（Int） |

灵魂营火在 Java 也是同名方块实体，Bedrock 实体 id 仍为 `Campfire`；方块名决定
Bedrock 方块状态。

## 降级

槽位不在 0–3 的物品跳过；`CookingTimes` 不是 Int 数组时省略时间字段；单条物品失败
记 `unknown_item`，其余槽位继续。
