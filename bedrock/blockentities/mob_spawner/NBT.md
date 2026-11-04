# 刷怪笼的 NBT

Bedrock 实体 ID 是 `MobSpawner`，Java 目标 ID 是 `minecraft:mob_spawner`。

## 源字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `Delay`、`MinSpawnDelay`、`MaxSpawnDelay` | 数值 | 生成间隔 |
| `SpawnCount`、`MaxNearbyEntities` | 数值 | 数量限制 |
| `RequiredPlayerRange`、`SpawnRange` | 数值 | 范围限制 |
| `EntityIdentifier` | String | 要生成的实体标识 |

## 转换

七个计时/范围字段在 Java 1.21.4 都是 Short，读取后统一收窄。
`EntityIdentifier` 嵌进 `SpawnData.entity.id`；`SpawnPotentials` 固定写空列表。
Bedrock 只保存一个候选实体，Java 的多候选生成配置不重建。

## 降级

缺字段不写；`EntityIdentifier` 缺失时只保留计时字段。
生成的实体标识不在这里做 Java 改名。
