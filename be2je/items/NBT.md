# 物品的 NBT 结构

Bedrock 物品堆叠是一个 Compound，Java 1.21.4 用 `{id, count, components}` 表示。
箱子、掉落物、展示框、营火等都共用这份转换。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `Name` | String | Bedrock 物品标识 |
| `Count` | Byte | 数量 |
| `Damage` | Short | 旧版 data value；药水是种类编号 |
| `Slot` | Byte | 所在槽位 |
| `Block` | Compound | 方块物品的 `name`、`states`、`version` |
| `tag` | Compound | 物品附加数据 |

`tag` 里读取的字段：

| 字段 | 用途 |
|---|---|
| `Damage` | 耐久损耗 → `minecraft:damage` |
| `ench` | 附魔列表 → `minecraft:enchantments` / `stored_enchantments` |
| `display.Name`、`display.Lore` | 自定义名称与 lore |
| `customColor` | 皮革颜色 → `minecraft:dyed_color {rgb}` |
| `Unbreakable` | `minecraft:unbreakable` |
| `RepairCost` | `minecraft:repair_cost` |
| `Items` | 嵌套容器 → `minecraft:container` |
| `pages`、`title`、`author`、`generation` | 成书/可编辑书 |

## 转换顺序

1. 方块物品先走 `neomega-blocks` 的 data value / `Block` 转换，保持和方块表一致；
2. 药水箭（`arrow` 且 `Damage > 0`）特殊处理；
3. 其余查 `third_party` 的物品表；旧名 `scute`、`record_*` 先归一；
4. 药水、成书等仍按已知类型兜底。

## 失败

名称为空、空气或数量非正时返回 nil，不算失败。物品表查不到、药水/附魔编号越界、
书页类型错误、容器列表类型错误时返回 nil，并把物品名交给 `ReportItem`；
区块诊断记 `unknown_item`。容器里转不出的条目逐条跳过，容器本身保留。

## 预览物品 ID

`ConvertItemWithResolver` 可在旧 ID 解析后、组件转换前调用只读 `IDResolver`。回调返回目标物品 ID 及是否为方块物品；空 ID 继续报告 `unknown_item`。数量、空气、药水/附魔/书本等校验仍走原流程。nil 回调保持默认导出行为。
