# Java 末地折跃门 → Bedrock EndGateway

Java 的出口坐标是 `exit_portal` IntArray，Bedrock 是 `ExitPortal` 列表。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `exit_portal` | `ExitPortal`，三个 Int 的列表 |
| `Age` | `Age`（Int） |

只有长度正好为 3 的 `exit_portal` 才写；方块状态只决定 Bedrock 方块，实体 id 固定
`EndGateway`。

## 降级

`exit_portal` 缺失或长度不对时省略 `ExitPortal`；`Age` 不是整数时省略。
