package export

import (
	"reflect"
	"testing"

	cellerbedrock "celler/bedrock"
	"neomega-converter-core/javaout"
	nbt "neomega-nbt/presets/v1"
)

func TestConvertLevel(t *testing.T) {
	level := cellerbedrock.Level{NBT: nbt.Document{Value: nbt.Compound{
		"LevelName": "测试", "SpawnX": int32(-12), "SpawnY": int32(100), "SpawnZ": int32(33),
		"RandomSeed": int64(-9876543210), "GameType": int32(1), "Difficulty": int32(3), "commandsAllowed": int8(1),
		"Time": int64(12345), "currentTick": int64(999), "SpawnRotation": float32(90),
		"IsHardcore": int8(1), "LastPlayed": int64(1700000000), "GenerateStructures": int8(0),
		"rainLevel": float32(0.5), "lightningLevel": float32(0), "rainTime": int32(100), "lightningTime": int32(200),
		"keepinventory": int8(1), "randomtickspeed": int32(5),
	}}}
	document, report := ConvertLevel(level)
	if len(report.DefaultedFields) != 0 {
		t.Fatalf("report=%v", report)
	}
	world := openJava(t)
	if err := world.WriteLevel(document); err != nil {
		t.Fatal(err)
	}
	got, err := world.ReadLevel()
	if err != nil {
		t.Fatal(err)
	}
	data := got.Value["Data"].(nbt.Compound)
	for key, want := range (nbt.Compound{
		"LevelName": "测试", "SpawnX": int32(-12), "SpawnY": int32(100), "SpawnZ": int32(33),
		"GameType": int32(1), "Difficulty": int8(3), "Time": int64(999), "DayTime": int64(12345), "allowCommands": int8(1),
		"hardcore": int8(1), "LastPlayed": int64(1700000000000), "SpawnAngle": float32(90),
		"raining": int8(1), "thundering": int8(0), "rainTime": int32(100), "thunderTime": int32(200),
		"DataVersion": javaout.DataVersion, "version": int32(19133), "initialized": int8(1),
		"Version":   nbt.Compound{"Id": javaout.DataVersion, "Name": "1.21.4", "Series": "main", "Snapshot": int8(0)},
		"DataPacks": nbt.Compound{"Enabled": []string{"vanilla"}, "Disabled": []string{}},
	}) {
		if !reflect.DeepEqual(data[key], want) {
			t.Errorf("%s=%#v want=%#v", key, data[key], want)
		}
	}
	rules := data["GameRules"].(nbt.Compound)
	if rules["keepInventory"] != "true" || rules["randomTickSpeed"] != int32(5) {
		t.Fatalf("gamerules: %#v", rules)
	}
	settings := data["WorldGenSettings"].(nbt.Compound)
	if settings["seed"] != int64(-9876543210) || settings["generate_features"] != int8(0) || settings["bonus_chest"] != int8(0) {
		t.Fatal(settings)
	}
	dimensions := settings["dimensions"].(nbt.Compound)
	for _, test := range []struct{ key, kind, noise, preset string }{
		{"minecraft:overworld", "minecraft:overworld", "minecraft:overworld", "minecraft:overworld"},
		{"minecraft:the_nether", "minecraft:the_nether", "minecraft:nether", "minecraft:nether"},
		{"minecraft:the_end", "minecraft:the_end", "minecraft:end", ""},
	} {
		dim := dimensions[test.key].(nbt.Compound)
		generator := dim["generator"].(nbt.Compound)
		biome := generator["biome_source"].(nbt.Compound)
		if dim["type"] != test.kind || generator["settings"] != test.noise || generator["type"] != "minecraft:noise" {
			t.Fatal(dim)
		}
		if test.preset == "" {
			if biome["type"] != "minecraft:the_end" {
				t.Fatal(biome)
			}
		} else if biome["type"] != "minecraft:multi_noise" || biome["preset"] != test.preset {
			t.Fatal(biome)
		}
	}
}

func TestConvertLevelDefaults(t *testing.T) {
	document, report := ConvertLevel(cellerbedrock.Level{NBT: nbt.Document{Value: nbt.Compound{"SpawnX": "bad"}}})
	wantFields := []string{
		"LevelName", "SpawnX", "SpawnY", "SpawnZ", "SpawnRotation", "RandomSeed", "GameType",
		"Difficulty", "Time", "currentTick", "commandsAllowed", "LastPlayed", "rainTime", "lightningTime",
	}
	if !reflect.DeepEqual(report.DefaultedFields, wantFields) {
		t.Fatal(report)
	}
	data := document.Value["Data"].(nbt.Compound)
	for key, want := range (nbt.Compound{"LevelName": "New World", "SpawnX": int32(0), "SpawnY": int32(64), "SpawnZ": int32(0), "GameType": int32(0), "Difficulty": int8(2), "Time": int64(0), "allowCommands": int8(0)}) {
		if data[key] != want {
			t.Errorf("%s=%v want=%v", key, data[key], want)
		}
	}
	if data["WorldGenSettings"].(nbt.Compound)["seed"] != int64(0) {
		t.Fatal("seed default")
	}
}

func TestConvertLevelFlat(t *testing.T) {
	layers := `{"biome_id":1,"block_layers":[{"block_name":"minecraft:bedrock","count":1},{"block_name":"stone","count":2}]}`
	document, _ := ConvertLevel(cellerbedrock.Level{NBT: nbt.Document{Value: nbt.Compound{
		"Generator": int32(2), "FlatWorldLayers": layers,
	}}})
	data := document.Value["Data"].(nbt.Compound)
	settings := data["WorldGenSettings"].(nbt.Compound)
	overworld := settings["dimensions"].(nbt.Compound)["minecraft:overworld"].(nbt.Compound)
	if overworld["type"] != "minecraft:overworld" {
		t.Fatalf("dimension: %#v", overworld)
	}
	generator := overworld["generator"].(nbt.Compound)
	if generator["type"] != "minecraft:flat" {
		t.Fatalf("generator: %#v", generator)
	}
	flat := generator["settings"].(nbt.Compound)
	if flat["biome"] != "minecraft:plains" {
		t.Fatalf("biome: %#v", flat["biome"])
	}
	list, ok := flat["layers"].([]nbt.Compound)
	if !ok || len(list) != 2 || list[0]["block"] != "minecraft:bedrock" ||
		list[1]["block"] != "minecraft:stone" || list[1]["height"] != int32(2) {
		t.Fatalf("layers: %#v", flat["layers"])
	}
}
