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
| `PersistenceRequired` | `Persistent`；有自定义名称或本身不消失的实体也写 1 |
| `Age < 0` 或 `IsBaby` | `IsBaby` |
| `Passengers` | 递归转换，每条乘客单独占一个 digp/actorprefix |
| 所有实体 | `definitions: ["+minecraft:<id>"]` |
| `Health` | 有 `EntityDefs` 时写 `Attributes[minecraft:health]` 的 Base/Max/Current；没有表时不写，和 mcconv 缺少实体定义时的行为一致 |
| `equipment` | `Armor`（四件 list）、`Mainhand`/`Offhand`（单元素 list） |
| `ArmorItems`/`HandItems`（旧版） | 归一化成同样的 list，顺序 feet、legs、chest、head / main、off |
| `Item`、`Age` | 掉落物的 `Item`、`Age` |
| `oak_boat` 等 | `boat`/`chest_boat` + `Variant` |
| 羊的 `Color`、`Sheared` | `Color`、`Sheared` |
| 马的 `Variant` | `Variant`/`MarkVariant`，并加 horse 的 `definitions` 组 |
| 画的 `variant`、`facing` | `Motif`、`Direction`、`block_pos` |
| 运输船/箱子矿车/漏斗矿车的 `Items`、`LootTable` | `ChestItems`、`LootTable` |

## 降级

瞬时实体跳过；未知标识通过 `unknown_entity` 计数。村民交易、乘骑、宠物主人、
盔甲架姿势等不转换。`digp` 键由 UUID 或坐标哈希确定性生成。

## 展示框

Java 的展示框是实体，Bedrock 是 `frame`/`glow_frame` 方块加 `ItemFrame`/`GlowItemFrame`
方块实体。`je2be/frame.go` 按 `block_pos`（旧版是 `TileX/Y/Z`）把方块替换成
`frame`，`facing_direction` 取 `Facing`，`item_frame_map_bit` 在物品是已填充地图时写 1，
并把 `Item`、`ItemRotation`（Java 的 0–7 转成 45° 的倍数）、`ItemDropChance` 写进方块实体。

只替换空气；水下展示框需要额外水层，当前跳过以免丢水。位置不在当前柱子或方块已被占用时
跳过这条实体，不报错。

实体定义表可以从 mcconv 生成的 `bedrock_entities.json` 用 `LoadEntityDefs` 载入，
命令行对应 `java-to-bedrock -entity-defs <file>`。没有表时只写 `definitions: ["+id"]`，
不写 health 的 `Attributes`，与 mcconv 找不到实体定义时的降级一致。
