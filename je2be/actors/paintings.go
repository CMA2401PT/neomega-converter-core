package actors

import (
	"math"
	"strings"
)

// paintingSizes 是 Java motif 的宽高（方块）。Bedrock 用 Motif 名字和 block_pos。
var paintingSizes = map[string][2]int{
	"kebab": {1, 1}, "aztec": {1, 1}, "alban": {1, 1}, "aztec2": {1, 1}, "bomb": {1, 1}, "plant": {1, 1},
	"wasteland": {1, 1}, "meditative": {1, 1}, "pool": {2, 1}, "courbet": {2, 1}, "sea": {2, 1},
	"sunset": {2, 1}, "creebet": {2, 1}, "wanderer": {1, 2}, "graham": {1, 2}, "prairie_ride": {1, 2},
	"match": {2, 2}, "bust": {2, 2}, "stage": {2, 2}, "void": {2, 2}, "skull_and_roses": {2, 2},
	"wither": {2, 2}, "baroque": {2, 2}, "humble": {2, 2}, "earth": {2, 2}, "fire": {2, 2},
	"water": {2, 2}, "wind": {2, 2}, "fighters": {4, 2}, "changing": {4, 2}, "finding": {4, 2},
	"lowmist": {4, 2}, "passage": {4, 2}, "skeleton": {4, 3}, "donkey_kong": {4, 3}, "pointer": {4, 4},
	"pigscene": {4, 4}, "burning_skull": {4, 4}, "orb": {4, 4}, "unpacked": {4, 4}, "backyard": {3, 4},
	"pond": {3, 4}, "bouquet": {3, 3}, "cavebird": {3, 3}, "cotan": {3, 3}, "endboss": {3, 3},
	"fern": {3, 3}, "owlemons": {3, 3}, "sunflowers": {3, 3}, "tides": {3, 3}, "dennis": {3, 3},
}

func paintingSize(name string) (int, int) {
	if size, ok := paintingSizes[name]; ok {
		return size[0], size[1]
	}
	return 1, 1
}

// paintingBlockPos 把 Java 的画中心坐标换算成 Bedrock 的锚点方块坐标。
// facing：0 南(+z)、1 西(-x)、2 北(-z)、3 东(+x)。
func paintingBlockPos(center []float32, facing, width, height int) []int32 {
	fx := []float64{0, -1, 0, 1}[facing&3]
	fz := []float64{1, 0, -1, 0}[facing&3]
	// 面向方向的逆时针方向。
	cx, cz := fz, -fx
	x := float64(center[0]) + fx*0.46875
	y := float64(center[1])
	z := float64(center[2]) + fz*0.46875
	if width%2 == 0 {
		x -= cx * 0.5
		z -= cz * 0.5
	}
	if height%2 == 0 {
		y -= 0.5
	}
	return []int32{int32(math.Floor(x)), int32(math.Floor(y)), int32(math.Floor(z))}
}

func pascalCase(value string) string {
	out := make([]rune, 0, len(value))
	upper := true
	for _, r := range value {
		if r == '_' {
			upper = true
			continue
		}
		if upper {
			r = []rune(strings.ToUpper(string(r)))[0]
			upper = false
		}
		out = append(out, r)
	}
	return string(out)
}
