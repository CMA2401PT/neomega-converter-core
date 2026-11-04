package tables

import "testing"

func TestVersions(t *testing.T) {
	java, bedrock := Versions()
	if java != "1.21.4" || bedrock != "1.21.50" {
		t.Fatalf("版本 = %q/%q，期望 1.21.4/1.21.50", java, bedrock)
	}
}

func TestItem(t *testing.T) {
	for _, test := range []struct {
		name    string
		bedrock string
		data    int32
		java    string
	}{
		{"identity", "minecraft:diamond_sword", 0, "minecraft:diamond_sword"},
		{"bed colour from data", "minecraft:bed", 14, "minecraft:red_bed"},
		{"bed default colour", "minecraft:bed", 0, "minecraft:white_bed"},
		{"renamed item", "minecraft:undyed_shulker_box", 0, "minecraft:shulker_box"},
		{"block item", "minecraft:oak_log", 0, "minecraft:oak_log"},
		{"namespace filled in", "diamond_sword", 0, "minecraft:diamond_sword"},
	} {
		t.Run(test.name, func(t *testing.T) {
			java, ok := Item(test.bedrock, test.data)
			if !ok || java != test.java {
				t.Fatalf("Item(%q, %d) = %q, %v；期望 %q, true", test.bedrock, test.data, java, ok, test.java)
			}
		})
	}
}

func TestItemDataFallback(t *testing.T) {
	// 具体 data 没有条目时回退 data 0，和 mcconv 的查表顺序一致。
	java, ok := Item("minecraft:bed", 99)
	if !ok || java != "minecraft:white_bed" {
		t.Fatalf("Item 回退 = %q, %v；期望 minecraft:white_bed, true", java, ok)
	}
}

func TestUnknownItem(t *testing.T) {
	if java, ok := Item("minecraft:not_a_real_item", 0); ok {
		t.Fatalf("未知物品返回了 %q", java)
	}
}

func TestBiome(t *testing.T) {
	for _, test := range []struct {
		id   int32
		java string
	}{
		{1, "minecraft:plains"},
		{16, "minecraft:beach"},
	} {
		java, ok := Biome(test.id)
		if !ok || java != test.java {
			t.Fatalf("Biome(%d) = %q, %v；期望 %q, true", test.id, java, ok, test.java)
		}
	}
	if java, ok := Biome(1 << 20); ok {
		t.Fatalf("未知群系返回了 %q", java)
	}
}

func TestBannerPattern(t *testing.T) {
	if java, ok := BannerPattern("bo"); !ok || java != "minecraft:border" {
		t.Fatalf("BannerPattern(bo) = %q, %v", java, ok)
	}
	if _, ok := BannerPattern("nope"); ok {
		t.Fatal("未知图案代码被命中")
	}
}

func TestKnownEntity(t *testing.T) {
	if !KnownEntity("minecraft:chicken") {
		t.Fatal("minecraft:chicken 应存在")
	}
	if !KnownEntity("chicken") {
		t.Fatal("无命名空间的名字也应识别")
	}
	if KnownEntity("minecraft:not_a_real_entity") {
		t.Fatal("未知实体被命中")
	}
}

func TestSources(t *testing.T) {
	sources := Sources()
	if len(sources) != 2 {
		t.Fatalf("来源数 = %d，期望 2", len(sources))
	}
	if sources[0].Name != "GeyserMC/mappings" || sources[0].Commit == "" || sources[0].License != "MIT" {
		t.Fatalf("第一个来源 = %+v", sources[0])
	}
	if sources[1].Name != "pmmp/BedrockData" || sources[1].License != "CC0-1.0" {
		t.Fatalf("第二个来源 = %+v", sources[1])
	}
}

func TestReverseLookups(t *testing.T) {
	for _, test := range []struct {
		java    string
		bedrock string
		data    int32
	}{
		{"minecraft:diamond_sword", "minecraft:diamond_sword", 0},
		{"minecraft:red_bed", "minecraft:bed", 14},
		{"minecraft:shulker_box", "minecraft:undyed_shulker_box", 0},
	} {
		bedrock, data, ok := BedrockItem(test.java)
		if !ok || bedrock != test.bedrock || data != test.data {
			t.Fatalf("BedrockItem(%q) = %q, %d, %v", test.java, bedrock, data, ok)
		}
	}
	if _, _, ok := BedrockItem("minecraft:not_a_real_item"); ok {
		t.Fatal("unknown Java item resolved")
	}
	if id, ok := BedrockBiome("minecraft:beach"); !ok || id != 16 {
		t.Fatalf("BedrockBiome(beach) = %d, %v", id, ok)
	}
	if code, ok := BannerPatternCode("minecraft:border"); !ok || code != "bo" {
		t.Fatalf("BannerPatternCode(border) = %q, %v", code, ok)
	}
}
