# Java 实体 → Bedrock actor

Java 实体存在 `entities/` region；Bedrock 用 `digp` 索引和 `actorprefix<id>` 记录。
`je2be/actors` 只转通用字段和掉落物、船、下落方块。

## 字段

| Java | Bedrock |
|---|---|
| `id` | `identifier`，经 `mapping.BedrockEntityID` 改名 |
| `Pos`、`Motion` | 同名 Float 列表 |
| `Rotation` | 同名 Float 列表 |
| `UUID` | `UniqueID`，取 IntArray 的后两个 Int |
| `CustomName` | `CustomName`，JSON 转 § |
| `Tags` | `Tags` |
| `PersistenceRequired` | `Persistent` |
| `Age = -24000` | `IsBaby` |
| `Health` | `Attributes[minecraft:health]` |
| `equipment` | `Armor{Head…}`、`Mainhand`、`Offhand` |
| `Item`、`Age` | 掉落物的 `Item`、`Age` |
| `oak_boat` 等 | `boat`/`chest_boat` + `Variant` |

## 降级

瞬时实体跳过；未知标识通过 `unknown_entity` 计数。村民交易、乘骑、宠物主人、
盔甲架姿势等不转换。`digp` 键由 UUID 或坐标哈希确定性生成。
