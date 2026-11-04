package java_test

import (
	"reflect"
	"testing"

	"celler"
	"neomega-blocks/ir"
	bedrockentities "neomega-converter-core/bedrock/blockentities"
	bedrockbanner "neomega-converter-core/bedrock/blockentities/banner"
	bedrockchest "neomega-converter-core/bedrock/blockentities/chest"
	bedrockfurnace "neomega-converter-core/bedrock/blockentities/furnace"
	bedrocksign "neomega-converter-core/bedrock/blockentities/sign"
	javaentities "neomega-converter-core/java/blockentities"
	javabanner "neomega-converter-core/java/blockentities/banner"
	javachest "neomega-converter-core/java/blockentities/chest"
	javafurnace "neomega-converter-core/java/blockentities/furnace"
	javasign "neomega-converter-core/java/blockentities/sign"
	nbt "neomega-nbt/presets/v1"
)

func TestChestRoundTrip(t *testing.T) {
	javaState := ir.IRBlock(`minecraft:chest ["facing"="north","type"="single"]`)
	forward, err := bedrockchest.Convert(bedrockentities.Input{
		Base:     javaState,
		WorldPos: celler.BlockPos{X: 4, Y: 64, Z: -2},
		Source: nbt.Compound{
			"Items": []nbt.Compound{
				{"Name": "minecraft:oak_log", "Count": int8(2), "Slot": int8(0)},
				{"Name": "minecraft:diamond_sword", "Count": int8(1), "Slot": int8(1)},
			},
			"CustomName": "箱子",
			"LootTable":  "loot_tables/chests/simple_dungeon.json",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	back, err := javachest.Convert(javaentities.Input{
		SourceState: javaState,
		Base:        "minecraft:chest []",
		WorldPos:    celler.BlockPos{X: 4, Y: 64, Z: -2},
		Source:      forward.BlockEntity,
	})
	if err != nil {
		t.Fatal(err)
	}
	if back.BlockEntity["id"] != "Chest" || back.BlockEntity["CustomName"] != "箱子" ||
		back.BlockEntity["LootTable"] != "loot_tables/chests/simple_dungeon.json" {
		t.Fatalf("metadata: %#v", back.BlockEntity)
	}
	items, _ := back.BlockEntity["Items"].([]nbt.Compound)
	if len(items) != 2 || items[0]["Name"] != "minecraft:oak_log" || items[0]["Count"] != int8(2) ||
		items[1]["Name"] != "minecraft:diamond_sword" {
		t.Fatalf("items: %#v", items)
	}
}

func TestFurnaceRoundTrip(t *testing.T) {
	javaState := ir.IRBlock(`minecraft:furnace ["facing"="north","lit"="false"]`)
	forward, err := bedrockfurnace.Convert(bedrockentities.Input{
		Base: javaState,
		Source: nbt.Compound{
			"BurnTime": int16(120), "CookTime": int16(40), "BurnDuration": int16(300),
			"Items": []nbt.Compound{{"Name": "minecraft:iron_ore", "Count": int8(1), "Slot": int8(0)}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	back, err := javafurnace.Convert(javaentities.Input{
		SourceState: javaState,
		Base:        "minecraft:furnace []",
		Source:      forward.BlockEntity,
	})
	if err != nil {
		t.Fatal(err)
	}
	if back.BlockEntity["id"] != "Furnace" || back.BlockEntity["BurnTime"] != int16(120) ||
		back.BlockEntity["CookTime"] != int16(40) || back.BlockEntity["BurnDuration"] != int16(300) {
		t.Fatalf("timers: %#v", back.BlockEntity)
	}
}

func TestBannerRoundTrip(t *testing.T) {
	forward, err := bedrockbanner.Convert(bedrockentities.Input{
		Base: `minecraft:white_banner ["rotation"="0"]`,
		Source: nbt.Compound{
			"Base": int32(14), // 红色
			"Patterns": []nbt.Compound{
				{"Pattern": "bo", "Color": int32(15)},
				{"Pattern": "cre", "Color": int32(0)},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	back, err := javabanner.Convert(javaentities.Input{
		SourceState: forward.State,
		Base:        `minecraft:red_banner []`,
		Source:      forward.BlockEntity,
	})
	if err != nil {
		t.Fatal(err)
	}
	if back.BlockEntity["Base"] != int32(14) {
		t.Fatalf("base: %#v", back.BlockEntity)
	}
	patterns, _ := back.BlockEntity["Patterns"].([]nbt.Compound)
	want := []nbt.Compound{{"Pattern": "bo", "Color": int32(15)}, {"Pattern": "cre", "Color": int32(0)}}
	if !reflect.DeepEqual(patterns, want) {
		t.Fatalf("patterns: %#v", patterns)
	}
}

func TestSignRoundTrip(t *testing.T) {
	forward, err := bedrocksign.Convert(bedrockentities.Input{
		Base: `minecraft:oak_sign ["rotation"="0"]`,
		Source: nbt.Compound{
			"FrontText": nbt.Compound{
				"Text": "首行\n第二行", "SignTextColor": int32(0xB02E26) | -16777216, "IgnoreLighting": int8(1),
			},
			"IsWaxed": int8(1),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	back, err := javasign.Convert(javaentities.Input{
		SourceState: `minecraft:oak_sign ["rotation"="0"]`,
		Base:        `minecraft:oak_sign []`,
		Source:      forward.BlockEntity,
	})
	if err != nil {
		t.Fatal(err)
	}
	if back.BlockEntity["id"] != "Sign" || back.BlockEntity["IsWaxed"] != int8(1) {
		t.Fatalf("fields: %#v", back.BlockEntity)
	}
	front, _ := back.BlockEntity["FrontText"].(nbt.Compound)
	if front["Text"] != "首行\n第二行" || front["IgnoreLighting"] != int8(1) {
		t.Fatalf("front: %#v", front)
	}
}
