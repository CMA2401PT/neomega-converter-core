package items

import (
	nbt "neomega-nbt/presets/v1"
	"reflect"
	"testing"
)

func TestItemResolverRetainsValidation(t *testing.T) {
	source := nbt.Compound{"Name": "example:new_tool", "Count": int8(1), "Damage": int16(0), "tag": nbt.Compound{"Damage": int32(12), "display": nbt.Compound{"Name": "test"}}}
	var reported []string
	report := func(name string) { reported = append(reported, name) }
	if got := ConvertItemReported(source, report); got != nil || len(reported) != 1 {
		t.Fatal("fixture must be unknown to legacy table")
	}
	reported = nil
	resolver := func(raw nbt.Compound, id string, blockItem bool) (string, bool) {
		return "minecraft:diamond_sword", false
	}
	got := ConvertItemWithResolver(source, report, resolver)
	if got == nil || got["id"] != "minecraft:diamond_sword" || len(reported) != 0 {
		t.Fatalf("got=%+v reported=%v", got, reported)
	}
	if comps := got["components"].(nbt.Compound); comps["minecraft:damage"] != int32(12) || comps["minecraft:custom_name"] == nil {
		t.Fatalf("metadata: %+v", comps)
	}
	if !reflect.DeepEqual(ConvertItemReported(source, nil), ConvertItemWithResolver(source, nil, nil)) {
		t.Fatal("nil resolver changed legacy behavior")
	}
	source["tag"].(nbt.Compound)["ench"] = []nbt.Compound{{"id": int16(-1), "lvl": int16(1)}}
	if ConvertItemWithResolver(source, report, resolver) != nil || len(reported) != 1 {
		t.Fatal("resolver bypassed invalid component rejection")
	}
	for _, empty := range []nbt.Compound{{"Name": "minecraft:air", "Count": int8(1)}, {"Name": "example:new_tool", "Count": int8(0)}} {
		if ConvertItemWithResolver(empty, report, func(nbt.Compound, string, bool) (string, bool) {
			t.Fatal("resolver called for empty stack")
			return "", false
		}) != nil {
			t.Fatal("empty stack retained")
		}
	}
}
