# Java 讲台 → Bedrock Lectern

Java 讲台用 `Book` 和 `Page`；Bedrock 用 `book`、`hasBook`、`page`。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `Book` | `book`，经 `je2be/items` 转换并删掉 `Slot` |
| `Page` | `page`（Int），只在书转换成功时写 |

有书时写 `hasBook=1`；没有书时不写 `book`/`hasBook`/`page`。

## 降级

`Book` 缺失或物品转不出时按空讲台输出；`Page` 不是整数时省略 `page`。
