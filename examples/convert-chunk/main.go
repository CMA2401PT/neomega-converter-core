// 从工作区根目录运行：go run ./neomega-converter-core/examples/convert-chunk
// 在内存中构造蓝床与含水台阶，展示整根区块转换及诊断读取。
package main

import (
	"fmt"
	"log"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converter "neomega-converter-core"
	"neomega-converter-core/be2je"
	nbt "neomega-nbt/presets/v1"
)

func main() {
	// 1. 区块坐标 (0,0)、主世界维度 0；section Y=0 覆盖世界高度 0..15。
	// 条目版本与 section 编码版本是两回事；前者随方块保存，后者决定字节布局。
	const blockVersion int32 = 18168865
	const sectionVersion byte = 9
	var source *cellerbedrock.Chunk = cellerbedrock.NewChunk(celler.ChunkPos{}, 0)
	// 两层以空气初始化，所以未设置的其余格子仍为空气。
	var base *cellerbedrock.Layer = cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:air []"})
	var water *cellerbedrock.Layer = cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:air []"})
	// 2. At 接收 section 内坐标；Tidy 将属性顺序整理为可比较的规范文本。
	base.Blocks.Set(
		layer_storage.At(4, 4, 4),
		cellerbedrock.Block{
			State:   ir.IRBlock(`minecraft:bed ["direction"=3,"head_piece_bit"=false,"occupied_bit"=false]`).Tidy(),
			Version: blockVersion,
		},
	)
	base.Blocks.Set(
		layer_storage.At(2, 4, 4),
		cellerbedrock.Block{
			State:   ir.IRBlock(`minecraft:oak_slab ["minecraft:vertical_half"="bottom"]`).Tidy(),
			Version: blockVersion,
		},
	)
	water.Blocks.Set(
		layer_storage.At(2, 4, 4),
		cellerbedrock.Block{State: `minecraft:water ["liquid_depth"=0]`, Version: blockVersion},
	)
	// 3. SetSection 将内存层编码放回区块；第二层的水会合并为 Java 含水属性。
	if err := source.SetSection(&cellerbedrock.Section{
		Y:       0,
		Version: sectionVersion,
		Layers:  []*cellerbedrock.Layer{base, water},
	}); err != nil {
		log.Fatal(err)
	}
	// 床颜色存放在方块实体中；NBT 要求 color 为 Byte、绝对坐标为 Int。
	// 蓝色的染料编号是 11；本例位于原点区块，绝对坐标恰好等于局部坐标。
	if err := source.SetBlockEntities([]nbt.Document{{Value: nbt.Compound{
		"id": "Bed", "color": int8(11),
		"x": int32(4), "y": int32(4), "z": int32(4),
	}}}); err != nil {
		log.Fatal(err)
	}
	// 4. error 表示整根柱子不能转换；Diagnostics 保存仍能输出时的可恢复问题。
	var converted *converter.JavaChunk
	var err error
	converted, err = be2je.ConvertChunk(source)
	if err != nil {
		log.Fatal(err)
	}
	// 按床、台阶顺序读取；Sections 的键是 section Y，不是世界方块高度。
	for _, x := range []int{4, 2} {
		fmt.Println(converted.Sections[0].Blocks.Get(layer_storage.At(x, 4, 4)))
	}
	for _, diagnostic := range converted.Diagnostics {
		fmt.Println(diagnostic.Kind, diagnostic.Count, diagnostic.Err)
	}
}
