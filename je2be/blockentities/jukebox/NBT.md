# Java 唱片机 → Bedrock Jukebox

Java 和 Bedrock 都用单个 `RecordItem` 存唱片。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `RecordItem` | `RecordItem`，经 `je2be/items` 转换 |

转换后的物品会删掉 `Slot`，因为唱片机只存一件物品，Bedrock 不读这个字段。

## 降级

`RecordItem` 缺失或不是 compound 时只写空的 `Jukebox` 实体；物品转不出时跳过并记
`unknown_item`。
