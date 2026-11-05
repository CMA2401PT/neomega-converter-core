// Package chest 把 Java 箱子转成 Bedrock 的 Chest 方块实体。
package chest

import (
	"fmt"

	"neomega-converter-core/je2be/blockentities"
)

const (
	ID        = "minecraft:chest"
	TrappedID = "minecraft:trapped_chest"
)

// Java 的 type=left/right 是相对 facing 的位置，Bedrock 存配对坐标。
var dirOffset = map[string][2]int32{
	"north": {0, -1}, "south": {0, 1}, "west": {-1, 0}, "east": {1, 0},
}
var clockwise = map[string]string{"north": "east", "east": "south", "south": "west", "west": "north"}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	entity, err := blockentities.Common(input)
	if err != nil {
		return blockentities.Result{}, err
	}
	entity["id"] = "Chest"
	entity["Findable"] = int8(0)
	if typ, ok := blockentities.StateString(input.SourceState, "type"); ok && (typ == "left" || typ == "right") {
		facing, ok := blockentities.StateString(input.SourceState, "facing")
		if !ok {
			return blockentities.Result{}, fmt.Errorf("double chest requires facing")
		}
		next, ok := clockwise[facing]
		if !ok {
			return blockentities.Result{}, fmt.Errorf("unknown chest facing %q", facing)
		}
		offset := dirOffset[next]
		if typ == "right" {
			offset[0], offset[1] = -offset[0], -offset[1]
		}
		entity["pairx"] = input.WorldPos.X + offset[0]
		entity["pairz"] = input.WorldPos.Z + offset[1]
		lead := int8(0)
		if typ == "left" {
			lead = 1
		}
		entity["pairlead"] = lead
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}
