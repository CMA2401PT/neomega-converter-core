# 普通实体的 NBT 结构

Bedrock 的实体存在 `digp`（每柱 id 列表）和 `actorprefix<id>`（实体 NBT）记录里，
旧存档还可能带 0x32 柱内记录。Java 写到独立的 `entities/` region。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `identifier` | String | Bedrock 实体标识 |
| `Pos`、`Rotation`、`Motion` | List<Float> | 位置、朝向、速度 |
| `UniqueID` | Long | 实体唯一编号 |
| `CustomName` | String | 自定义名称，可能带 `§` |
| `Tags` | List<String> | 计分板标签 |
| `Persistent`、`IsBaby`、`Fire`、`Air` | Byte/Short | 状态 |
| `FallDistance` | Float | 下落距离 |
| `Attributes` | List<Compound> | 属性，读取 `minecraft:health` |
| `Armor` | List<Compound> | 四件装备，顺序 head、chest、legs、feet，空槽位是空物品 |
| `Mainhand`、`Offhand` | List<Compound> | 单元素列表 |
| `Item`、`Variant`、`FallingBlock` | Compound/Int | 掉落物、船木种、下落方块 |

## 转换

- `identifier` 经一张小的改名表（僵尸猪灵、卫道士、村民变种等）得到 Java id；
  其余同名，且必须在 `third_party` 的已知实体集合里。
- 通用字段写 `Pos`/`Motion`（Double）、`Rotation`（Float）、`UUID`（IntArray）、
  `CustomName`（JSON 文本组件）、`Tags`、`Fire`、`Air`、`fall_distance`、
  `PersistenceRequired`、`Age`（幼年 -24000）、`Health`、`equipment`。
- 掉落物写 `Item`、`Age`、`PickupDelay`；船按 `Variant` 选木种与筏；
  下落方块把 `FallingBlock` 转成 `BlockState`。

## 失败

瞬时实体（箭、火球、经验瓶等）直接跳过。缺 `Pos` 或已知但数据不完整时跳过，不报错。
未知标识记 `unknown_entity`，按标识合并。物品转不出时记 `unknown_item`。
村民交易、乘骑、宠物主人、盔甲架姿势等类型专属数据不转换。
