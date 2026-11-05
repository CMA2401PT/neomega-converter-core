# Java 饰纹罐 → Bedrock DecoratedPot

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `sherds` | 同名 String 列表，保留顺序 |
| `item` | `item`，经 `je2be/items` 转换并删掉 `Slot` |

## 降级

`sherds` 缺失或为空时不写；`item` 缺失或转换失败时只写陶片列表（失败并记
`unknown_item`）。方块状态只决定 Bedrock 方块，实体 id 固定 `DecoratedPot`。
