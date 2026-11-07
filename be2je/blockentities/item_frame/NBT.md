# 展示框的 NBT 结构

Bedrock 的展示框是方块 `minecraft:frame` / `minecraft:glow_frame` 加方块实体；
Java 的展示框是独立实体，所在格子是空气。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `Item` | Compound | 框内物品 |
| `ItemRotation` | Float | 旋转角度 |
| `ItemDropChance` | Float | 掉落概率 |
| 方块状态 `facing_direction` | Int | 0..5，下、上、北、南、西、东 |

## 转换

规则把方块状态改成空气，并返回一个 Java 实体：

- 实体 ID 由源方块决定：`frame` → `minecraft:item_frame`，`glow_frame` → `minecraft:glow_item_frame`；
- `Pos` 是方块中心沿朝向反方向偏移 0.46875，同时写 `TileX/TileY/TileZ`；
- `Facing` 直接使用 `facing_direction`；
- `UUID` 由世界坐标和维度确定性生成，重复导出得到相同实体；
- `Item` 经物品转换并去掉 `Slot`；`ItemRotation` 除以 45 取整到 0..7；掉落概率缺省 1。

## 降级

源状态不是 `frame` / `glow_frame`、或 `facing_direction` 不是 0..5 时返回 error。
物品转不出时保留空展示框并记 `unknown_item`。`Rotation` 固定写 `[0,0]`，
`Fixed`、`Invisible` 等字段不转换。

预览调用方可通过 `ConvertOptions.FrameItemResolver`（或 adapt 同名选项）按当前物品资源目录补充/校正框内物品 ID。名称、耐久等组件仍由原物品转换器生成；该选项只影响展示框，不改变箱子内容或默认导出入口。
