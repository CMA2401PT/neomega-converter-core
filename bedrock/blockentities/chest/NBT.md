# 箱子的 NBT 结构

Bedrock 与 Java 的箱子都用方块实体 NBT 保存内容物，但字段名、物品格式和双箱表示不同。
Bedrock 实体 ID 是 `Chest`，普通箱子和陷阱箱共用。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `Items` | List<Compound> | 物品列表，每项含 `Name`、`Count`、`Slot` 和可选 `tag` |
| `pairx` / `pairz` | Int | 双箱另一半的世界坐标 |
| `CustomName` | String | 自定义名称，可能带 `§` 格式码 |
| `LootTable` | String | 战利品表路径，如 `loot_tables/chests/simple_dungeon.json` |
| `LootTableSeed` | Long | 战利品表随机种子 |

## 转换

- 方块状态交给公共容器规则，物品经 `items.ConvertItemReported` 转换并保留 `Slot`。
- `CustomName` 经 `textcomponent.Styled` 变成 Java 文本组件。
- `LootTable` 去掉 `loot_tables/` 前缀和 `.json` 后缀，补 `minecraft:` 命名空间。
- 双箱按 `facing` 与 `pairx`/`pairz` 的相对位置决定 `type=left/right`；坐标不相邻时保持单箱。
- 目标方块实体 ID 为 `minecraft:chest`，陷阱箱为 `minecraft:trapped_chest`。

## 降级

物品转不出时逐条跳过并记 `unknown_item`；`Items` 不是 compound 列表返回 error；
配对坐标不是 Int、朝向不是水平方向或坐标不相邻时按单箱输出。不写 `keepPacked`。
