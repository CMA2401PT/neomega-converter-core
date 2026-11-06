package converter

import (
	"testing"

	"neomega-blocks/ir"
)

func TestDiagnosticSummary(t *testing.T) {
	summary := NewDiagnosticSummary()
	summary.Add([]Diagnostic{
		{Kind: UnknownBlock, State: ir.IRBlock("minecraft:b"), Count: 3},
		{Kind: UnknownBlock, State: ir.IRBlock("minecraft:a"), Count: 3},
		{Kind: UnknownBlock, State: ir.IRBlock("minecraft:c"), Count: 5},
		{Kind: UnknownItem, ItemID: "minecraft:x", Count: 2},
		{Kind: UnsupportedBlockEntity, BlockEntityID: "Beehive"},
		{Kind: NBTError},
	})
	if summary.ByKind[UnknownBlock] != 11 || summary.ByKind[UnknownItem] != 2 ||
		summary.ByKind[UnsupportedBlockEntity] != 1 || summary.ByKind[NBTError] != 1 {
		t.Fatalf("by kind: %v", summary.ByKind)
	}
	top := TopCounts(summary.UnknownStates, 2)
	if len(top) != 2 || top[0].Key != "minecraft:c" || top[1].Key != "minecraft:a" {
		t.Fatalf("top: %v", top)
	}
}
