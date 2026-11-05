# Java → Bedrock 的 NBT 转换总览

`java/` 是反方向：源是 Java NBT，目标是 Bedrock。方块状态经 `neomega-blocks` 的
`ConvertFromJava` 转成 NEMC/Bedrock 状态，物品、方块实体、实体和 `level.dat` 由
本目录的规则处理。

## 两个方向不共用类型

- `be2je/blockentities.Input` 的 Base 是 Java 状态、Source 是 Bedrock NBT；
- `je2be/blockentities.Input` 的 Base 是 Bedrock 状态、Source 是 Java NBT。

两种状态都是 `ir.IRBlock`，但属于不同 hash 空间。不要把一边的输出直接塞进另一边的
写入器；必须经过 `ConvertFromJava`（或 `ConvertFromBedrock`）。

## 逐项文档

| 转换 | Java 源字段 | 文档 |
|---|---|---|
| 方块状态 | 调色板 + `waterlogged` | [java 包说明](convert.go) 与 `be2je/blockentities` 的反向规则 |
| 物品 | `id`、`count`、`components` | [items](items/NBT.md) |
| 普通实体 | `id`、`Pos`、`Rotation`、`item`、`equipment` 等 | [actors](actors/NBT.md) |
| level.dat | `Data` 里的世界字段 | [export](export/NBT.md) |
| 箱子 | `Items`、`CustomName`、`LootTable`、`type` | [bedrock 箱子](../be2je/blockentities/chest/NBT.md) |
| 熔炉 | `Items`、`lit_time_remaining` 等 | [bedrock 熔炉](../be2je/blockentities/furnace/NBT.md) |
| 旗帜 | `patterns`、方块颜色 | [bedrock 旗帜](../be2je/blockentities/banner/NBT.md) |
| 告示牌 | `front_text`/`back_text` | [bedrock 告示牌](../be2je/blockentities/sign/NBT.md) |
| 其余方块实体 | 槽位、效果、计时、文字 | 对应 `be2je/blockentities/<type>/NBT.md` 的反向字段 |

## 写出

`je2be/export` 写 Bedrock 存档：subchunk、`0x31` 方块实体、`Data3D` 群系、
`digp`/`actorprefix` 实体、`0x2C`/`0x36` 记录和 `level.dat`。
群系按目标维度补齐到 24（主世界）或 16（下界/末地）段。
