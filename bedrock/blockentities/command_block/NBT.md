# 命令方块的 NBT

Bedrock 实体 ID 是 `CommandBlock`，循环和连锁命令方块也用同一个 ID；
Java 目标 ID 是 `minecraft:command_block`。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `Command` | String | 命令文本 |
| `auto`、`powered`、`TrackOutput`、`conditionMet` | Byte | 状态位 |
| `SuccessCount` | Int | 上次执行成功次数 |
| `LastOutput` | String | 上次输出，可能带 `§` 格式码 |

## 转换

状态位和 `Command` 原样复制，`SuccessCount` 统一写成 Int。
`LastOutput` 经 `textcomponent.Styled` 变成 Java 文本组件。

## 降级

缺字段不写。命令语言本身不翻译：Bedrock 命令在 Java 里不保证能执行。
