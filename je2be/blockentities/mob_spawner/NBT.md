# Java 刷怪笼 → Bedrock MobSpawner

Java 和 Bedrock 的计时字段同名，实体标识的位置不同。

## 源字段（Java）

| 字段 | 转换 |
|---|---|
| `Delay`、`MinSpawnDelay`、`MaxSpawnDelay`、`SpawnCount`、`MaxNearbyEntities`、`RequiredPlayerRange`、`SpawnRange` | 同名 Short |
| `SpawnData.entity.id` | `EntityIdentifier`，经 `mapping.BedrockEntityID` 改名 |

## 降级

计时字段不是整数时跳过；`SpawnData` 缺失时只写计时字段和空实体标识。
`SpawnPotentials`、`SpawnData.entity` 的其余 NBT（装备、属性等）不转换，刷怪笼会
生成同类但无额外数据的实体。
