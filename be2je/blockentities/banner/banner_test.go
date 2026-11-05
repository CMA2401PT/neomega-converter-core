package banner

import (
	"reflect"
	"testing"

	"neomega-blocks/ir"
	"neomega-converter-core/be2je/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	for _, test := range []struct {
		name    string
		base    ir.IRBlock
		source  nbt.Compound
		want    ir.IRBlock
		wantErr bool
	}{
		{
			name:   "standing black",
			base:   `minecraft:white_banner ["rotation"="7"]`,
			source: nbt.Compound{"Base": int32(0)},
			want:   `minecraft:black_banner ["rotation"="7"]`,
		},
		{
			name:   "wall white",
			base:   `minecraft:red_wall_banner ["facing"="east"]`,
			source: nbt.Compound{"Base": int32(15)},
			want:   `minecraft:white_wall_banner ["facing"="east"]`,
		},
		{
			name:   "light gray",
			base:   `minecraft:white_banner ["rotation"="2"]`,
			source: nbt.Compound{"Base": int32(7)},
			want:   `minecraft:light_gray_banner ["rotation"="2"]`,
		},
		{
			name:    "missing",
			base:    `minecraft:white_banner ["rotation"="0"]`,
			source:  nbt.Compound{},
			wantErr: true,
		},
		{
			name:    "wrong type",
			base:    `minecraft:white_banner ["rotation"="0"]`,
			source:  nbt.Compound{"Base": int8(0)},
			wantErr: true,
		},
		{
			name:    "negative",
			base:    `minecraft:white_banner ["rotation"="0"]`,
			source:  nbt.Compound{"Base": int32(-1)},
			wantErr: true,
		},
		{
			name:    "above range",
			base:    `minecraft:white_banner ["rotation"="0"]`,
			source:  nbt.Compound{"Base": int32(16)},
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Convert(blockentities.Input{Base: test.base, Source: test.source})
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.State.Tidy() != test.want.Tidy() {
				t.Fatalf("got %s, want %s", got.State, test.want)
			}
			if got.BlockEntity == nil || got.BlockEntity["id"] != "minecraft:banner" {
				t.Fatalf("block entity: %v", got.BlockEntity)
			}
			if patterns, ok := got.BlockEntity["patterns"].([]nbt.Compound); !ok || len(patterns) != 0 {
				t.Fatalf("patterns: %v", got.BlockEntity["patterns"])
			}
		})
	}
}

func TestPatterns(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base: `minecraft:white_banner ["rotation"="0"]`,
		Source: nbt.Compound{
			"Base": int32(15),
			"Patterns": []nbt.Compound{
				{"Pattern": "bo", "Color": int32(15)},
				{"Pattern": "cre", "Color": int32(0)},
				{"Pattern": "glb", "Color": int32(4)},
				{"Pattern": "nope", "Color": int32(0)},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []nbt.Compound{
		{"pattern": "minecraft:border", "color": "white"},
		{"pattern": "minecraft:creeper", "color": "black"},
		{"pattern": "minecraft:globe", "color": "blue"},
	}
	if !reflect.DeepEqual(got.BlockEntity["patterns"], want) {
		t.Fatalf("patterns = %#v, want %#v", got.BlockEntity["patterns"], want)
	}
}

func TestPatternsWrongType(t *testing.T) {
	if _, err := Convert(blockentities.Input{
		Base:   `minecraft:white_banner ["rotation"="0"]`,
		Source: nbt.Compound{"Base": int32(15), "Patterns": "nope"},
	}); err == nil {
		t.Fatal("expected an error")
	}
}
