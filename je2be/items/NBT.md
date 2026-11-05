# Java 物品 → Bedrock 物品

Java 1.21.4 的物品是 `{id, count, components}`；Bedrock 用 `Name`、`Count`、`Damage`
和可选的 `Block`、`tag`。

## 源字段

| Java | Bedrock |
|---|---|
| `id` | `Name`，经 `third_party` 的 `java_items` 反查 |
| `count` | `Count`，Byte，超过 127 截断 |
| 方块物品 | `Block{name, states, version}`，状态经 `ConvertFromJava` |
| `minecraft:damage` | `tag.Damage` |
| `minecraft:custom_name`、`lore` | `tag.display.Name`、`tag.display.Lore`，JSON 文本转 § |
| `minecraft:enchantments` / `stored_enchantments` | `tag.ench` |
| `minecraft:dyed_color` | `tag.customColor`（不透明 ARGB） |
| `minecraft:unbreakable` | `tag.Unbreakable` |
| `minecraft:repair_cost` | `tag.RepairCost` |
| `minecraft:container` | `tag.Items`，递归转换并写 `Slot` |
| `minecraft:potion_contents` | 顶层 `Damage` |
| `written_book_content` / `writable_book_content` | `tag.pages`、`title`、`author`、`generation` |

## 降级

物品表查不到、药水编号无效等返回 nil，并通过 `ReportItem` 上报；调用方记
`unknown_item`。方块物品的 `Block` 只写默认状态，变体由 Java 物品 id 决定。
