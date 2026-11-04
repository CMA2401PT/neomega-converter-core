# 告示牌的 NBT 结构

Bedrock 实体 ID 是 `Sign` 或 `HangingSign`，方块形态（站立、墙上、悬挂）由方块状态决定。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `FrontText` / `BackText` | Compound | 两面的文字，含 `Text`（String）、`SignTextColor`（Int，ARGB）、`IgnoreLighting`（Byte） |
| `IsWaxed` | Byte | 是否涂蜡 |
| `Text`、`SignTextColor`、`IgnoreLighting` | 顶层字段 | 1.19.80 前后的旧版单面告示牌 |

`FrontText` 缺失时按旧版顶层字段读取正面。
`Text` 用 `\n` 分行，最多取前四行。

## 转换

每行经过 `textcomponent.Styled`，把 `§` 格式码转成 Java 的 JSON 文本组件。
`SignTextColor & 0xFFFFFF` 与 16 种染料取最近的颜色；`IgnoreLighting != 0` 写 `has_glowing_text`。
输出 `front_text`/`back_text` 各含 `messages`（四个字符串）、`color`、`has_glowing_text`，
以及顶层 `is_waxed`。目标实体 ID 为 `minecraft:sign` 或 `minecraft:hanging_sign`。

## 降级

`FrontText` 不是 compound 返回 error；缺失的背面写四个空行。
被过滤文字、`PersistFormatting`、`TextOwner` 不转换。
