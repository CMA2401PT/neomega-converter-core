# Java 潜影盒 → Bedrock ShulkerBox

Java 所有颜色的潜影盒共用方块实体 id `minecraft:shulker_box`；Bedrock 实体 id 是
`ShulkerBox`。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `Items` | `Items`，经 `je2be/items` 转换并保留 `Slot` |
| `CustomName` | JSON 文本转 § |
| `LootTable`、`LootTableSeed` | 同名/同路径转换 |
| 方块状态 `facing` | `facing`，映射 down=0、up=1、north=2、south=3、west=4、east=5 |

## 降级

`Items` 结构错误返回 error；单条物品失败记 `unknown_item`。`facing` 缺失或不是六个
方向之一时只写内容物，不写 `facing`，由游戏用默认朝向。
