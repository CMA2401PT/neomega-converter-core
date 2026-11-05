# Java 旗帜 → Bedrock Banner

Java 把旗帜底色放在方块名里，Bedrock 存在实体 NBT 的 `Base`；图案在 Java 是
`{pattern, color}` 列表，Bedrock 用 `{Pattern, Color}`。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| 方块名 `<color>_banner` | `Base = 15 - javaDyeIndex` |
| `patterns[].pattern` | `Patterns[].Pattern`，经 `third_party`/`mapping` 查代码 |
| `patterns[].color` | `Patterns[].Color`，同样取反的染料编号 |

Bedrock 的 `Base` 和图案 `Color` 都是 0 黑、15 白；Java 染料下标是 0 白、15 黑，
所以统一用 `15 - index`。`Type` 固定写 0。

## 降级

方块名里的颜色不认识时不写 `Base`；图案名查不到时跳过该层；图案颜色不认识时按
黑色（15）写。`patterns` 不是 compound 列表时按空列表处理。Java 的过滤文字和
`Translate` 不在这里处理。
