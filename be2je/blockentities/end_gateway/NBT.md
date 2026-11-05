# 末地折跃门的 NBT

Bedrock 实体 ID 是 `EndGateway`，Java 目标 ID 是 `minecraft:end_gateway`。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `ExitPortal` | List<Int> | 出口坐标，三个整数 |
| `Age` | 数值 | 存在时间 |

## 转换

`ExitPortal` 写成 Java 的 IntArray `exit_portal`；`Age` 写成 Long。
Bedrock 存的是列表，Java 要的是定长数组，这是两版格式差异最大的一处。

## 降级

`ExitPortal` 不是三个整数时省略该字段；其余字段缺失不写。
