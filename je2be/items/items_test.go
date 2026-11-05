package items

import (
	"reflect"
	"testing"

	"neomega-converter-core/mapping"
	nbt "neomega-nbt/presets/v1"
)

func TestConvertItem(t *testing.T) {
	for _, test := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "plain item",
			src:  `{id:"minecraft:diamond_sword",count:1}`,
			want: `{Name:"minecraft:diamond_sword",Count:1b}`,
		},
		{
			name: "block item keeps Block",
			src:  `{id:"minecraft:red_wool",count:3}`,
			want: `{Name:"minecraft:red_wool",Count:3b,Block:{name:"minecraft:red_wool",states:{},version:18166272}}`,
		},
		{
			name: "components",
			src:  `{id:"minecraft:diamond_sword",count:1,components:{"minecraft:damage":5,"minecraft:custom_name":'{"text":"剑"}',"minecraft:lore":['{"text":"一"}'],"minecraft:unbreakable":{},"minecraft:repair_cost":3,"minecraft:enchantments":{levels:{"minecraft:sharpness":2}}}}`,
			want: `{Name:"minecraft:diamond_sword",Count:1b,tag:{Damage:5,display:{Name:"剑",Lore:["一"]},ench:[{id:9s,lvl:2s}],Unbreakable:1b,RepairCost:3}}`,
		},
		{
			name: "dyed armour",
			src:  `{id:"minecraft:leather_chestplate",count:1,components:{"minecraft:dyed_color":{rgb:65280}}}`,
			want: `{Name:"minecraft:leather_chestplate",Count:1b,tag:{customColor:-16711936}}`,
		},
		{
			name: "container",
			src:  `{id:"minecraft:shulker_box",count:1,components:{"minecraft:container":[{slot:0,item:{id:"minecraft:oak_log",count:2}}]}}`,
			want: `{Name:"minecraft:undyed_shulker_box",Count:1b,Block:{name:"minecraft:undyed_shulker_box",states:{},version:18166272},tag:{Items:[{Name:"minecraft:oak_log",Count:2b,Slot:0b,Block:{name:"minecraft:oak_log",states:{"pillar_axis":"y"},version:18166272}}]}}`,
		},
		{
			name: "potion",
			src:  `{id:"minecraft:potion",count:1,components:{"minecraft:potion_contents":{potion:"minecraft:healing"}}}`,
			want: `{Name:"minecraft:potion",Count:1b,Damage:21s}`,
		},
		{
			name: "written book",
			src:  `{id:"minecraft:written_book",count:1,components:{"minecraft:written_book_content":{pages:[{raw:'{"text":"第一页","color":"green"}'}],title:{raw:'{"text":"标题"}'},author:"作者",generation:2}}}`,
			want: `{Name:"minecraft:written_book",Count:1b,tag:{pages:[{text:"§a第一页"}],title:"标题",author:"作者",generation:2}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := ConvertItem(decode(t, test.src), nil)
			if got == nil {
				t.Fatal("item skipped")
			}
			want := decode(t, test.want)
			if snbt(t, got) != snbt(t, want) {
				t.Fatalf("got %s, want %s", snbt(t, got), snbt(t, want))
			}
		})
	}
}

func TestConvertItemFailures(t *testing.T) {
	var reported []string
	report := func(name string) { reported = append(reported, name) }
	if got := ConvertItem(nbt.Compound{"id": "minecraft:not_a_real_item", "count": int32(1)}, report); got != nil {
		t.Fatalf("unknown item converted: %v", got)
	}
	if got := ConvertItem(nbt.Compound{"id": "minecraft:stone", "count": int32(0)}, report); got != nil {
		t.Fatalf("empty stack converted: %v", got)
	}
	if len(reported) != 1 || reported[0] != "minecraft:not_a_real_item" {
		t.Fatalf("reported: %v", reported)
	}
}

func TestPotionIndexMatches(t *testing.T) {
	index, ok := mapping.PotionIndex("minecraft:healing")
	if !ok || index != 21 {
		t.Fatalf("PotionIndex(healing) = %d, %v", index, ok)
	}
	if !reflect.DeepEqual(mapping.Potions[index], "healing") {
		t.Fatalf("potion table: %v", mapping.Potions[index])
	}
}

func decode(t *testing.T, text string) nbt.Compound {
	t.Helper()
	encoded, err := nbt.SNBTToNBT(text, nbt.OptSNBTToNBT{Encoding: nbt.EncodingLittleEndian})
	if err != nil {
		t.Fatal(err)
	}
	document, err := nbt.DecodeDocument(encoded, nbt.EncodingLittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	return document.Value
}

func snbt(t *testing.T, value nbt.Compound) string {
	t.Helper()
	text, err := nbt.EncodeToSNBT(value, nbt.OptEncodeToSNBT{SortMapKey: true})
	if err != nil {
		t.Fatal(err)
	}
	return text
}
