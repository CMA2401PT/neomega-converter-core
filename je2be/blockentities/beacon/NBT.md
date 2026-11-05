# Java 信标 → Bedrock Beacon

Java 用效果资源位置保存信标效果，Bedrock 用数字编号。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `primary_effect` | `primary`（Int） |
| `secondary_effect` | `secondary`（Int） |

当前映射：`speed=1`、`haste=3`、`strength=5`、`jump_boost=8`、`regeneration=10`、
`resistance=11`。方块名只决定 Bedrock 方块状态，实体 id 固定 `Beacon`。

## 降级

效果缺失、不是字符串或不在映射里时省略对应字段，由游戏按无效果处理。
