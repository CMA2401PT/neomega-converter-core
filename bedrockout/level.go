package bedrockout

import (
	cellerbedrock "celler/bedrock"
	nbt "neomega-nbt/presets/v1"
)

// storageVersion 是 level.dat 文件头与 StorageVersion 字段共用的版本。
const storageVersion = 10

// NewLevelData 返回 Bedrock level.dat 的版本字段；lastOpenedWithVersion 让游戏按 1.21.50 的格式打开。
// 调用方在其上填写世界内容，没写的字段由游戏取默认值。
func NewLevelData() nbt.Compound {
	return nbt.Compound{
		"StorageVersion":        int32(storageVersion),
		"lastOpenedWithVersion": []int32{1, 21, 50, 0},
	}
}

// Level 把 NewLevelData 填好的内容包成 level.dat。
func Level(data nbt.Compound) cellerbedrock.Level {
	return cellerbedrock.Level{Version: storageVersion, NBT: nbt.Document{Value: data}}
}
