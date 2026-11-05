# Java 告示牌 → Bedrock Sign

Java 用 `front_text`/`back_text` 两个 compound 保存两面文字；Bedrock 用
`FrontText`/`BackText`。方块形态（站立、墙上、悬挂）由方块状态决定，规则只改实体。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `front_text.messages` | `FrontText.Text`，四个 JSON 字符串各自转 § 后用 `\n` 连接并去掉末尾换行 |
| `front_text.color` | `FrontText.SignTextColor`，染料名转不透明 ARGB |
| `front_text.has_glowing_text` | `FrontText.IgnoreLighting` |
| `back_text.*` | `BackText.*`，规则同上 |
| `is_waxed` | `IsWaxed` |

另外写 `HideGlowOutline=0`、`PersistFormatting=1`、`TextOwner=""`。
Java 方块是 `minecraft:hanging_sign` 时目标实体 id 是 `HangingSign`，否则 `Sign`。

## 降级

某一面不是 compound 时按空文字、黑色、不发光处理；颜色不认识时用黑色。
Java 1.21 的 `filtered_messages` 不读，过滤后的文字丢失。
