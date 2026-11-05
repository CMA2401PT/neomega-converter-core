# 信标的 NBT

Bedrock 实体 ID 是 `Beacon`，Java 目标 ID 是 `minecraft:beacon`。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `primary` | Int | 主效果编号，0 表示没有 |
| `secondary` | Int | 辅助效果编号，0 表示没有 |

## 转换

信标只允许六种效果，编号与 Java 名称的对应固定：

| Bedrock | Java |
|---|---|
| 1 | `minecraft:speed` |
| 3 | `minecraft:haste` |
| 5 | `minecraft:strength` |
| 8 | `minecraft:jump_boost` |
| 10 | `minecraft:regeneration` |
| 11 | `minecraft:resistance` |

输出字段为 `primary_effect` / `secondary_effect`。

## 降级

未知编号、0 或字段缺失时不写对应效果，不报错。
