# Java 命令方块 → Bedrock CommandBlock

命令方块三兄弟共用 Java 实体 id `minecraft:command_block`，Bedrock 实体 id 是
`CommandBlock`。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `Command` | 同名 String，命令文本不做翻译 |
| `auto`、`powered`、`TrackOutput`、`conditionMet` | 同名 Byte |
| `SuccessCount` | `SuccessCount`（Int） |
| `LastOutput` | JSON 文本转 § |

## 降级

Java 的 `LastExecution`、`UpdateLastExecution` 在 Bedrock 没有直接对应，丢弃。
`LastOutput` 不是可解析的文本组件时按普通字符串写出。字段缺失时留空，由游戏补默认值。
