package bedrock

import (
	"testing"

	cellerbedrock "celler/bedrock"
	"neomega-converter-core"
	nbt "neomega-nbt/presets/v1"
)

func TestEntityRecords(t *testing.T) {
	source := biomeChunk(t, 0)
	encoded, err := nbt.EncodeDocuments([]nbt.Document{
		{Value: nbt.Compound{"identifier": "minecraft:zombie", "Pos": []float32{0, 1, 0}}},
		{Value: nbt.Compound{"identifier": "minecraft:not_a_real_entity", "Pos": []float32{0, 1, 0}}},
		{Value: nbt.Compound{"identifier": "minecraft:not_a_real_entity", "Pos": []float32{0, 1, 0}}},
	}, nbt.EncodingLittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	source.Records[string([]byte{cellerbedrock.TagEntities})] = encoded

	got, err := ConvertChunk(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entities) != 1 || got.Entities[0]["id"] != "minecraft:zombie" {
		t.Fatalf("entities: %#v", got.Entities)
	}
	var count uint64
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Kind == converter.UnknownEntity {
			if diagnostic.EntityID != "minecraft:not_a_real_entity" || diagnostic.Err == nil {
				t.Fatalf("diagnostic: %+v", diagnostic)
			}
			count += diagnostic.Count
		}
	}
	if count != 2 {
		t.Fatalf("unknown entity count = %d, diagnostics %+v", count, got.Diagnostics)
	}
}

func TestConvertChunkWithActors(t *testing.T) {
	source := biomeChunk(t, 0)
	got, err := ConvertChunkWithActors(source, []nbt.Compound{{
		"identifier": "minecraft:item", "Pos": []float32{0, 1, 0},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entities) != 1 || got.Entities[0]["id"] != "minecraft:item" {
		t.Fatalf("entities: %#v", got.Entities)
	}
}

func TestUnknownItemInActor(t *testing.T) {
	source := biomeChunk(t, 0)
	got, err := ConvertChunkWithActors(source, []nbt.Compound{{
		"identifier": "minecraft:item", "Pos": []float32{0, 1, 0},
		"Item": nbt.Compound{"Name": "minecraft:not_a_real_item", "Count": int8(1)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Kind == converter.UnknownItem && diagnostic.ItemID == "minecraft:not_a_real_item" && diagnostic.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics: %+v", got.Diagnostics)
	}
}
