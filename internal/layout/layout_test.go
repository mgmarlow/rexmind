package layout

import "testing"

func labels(n Node) []string {
	if n.Block != nil {
		return []string{n.Block.Label}
	}
	return append(labels(n.Split.Before), labels(n.Split.After)...)
}

func TestTile(t *testing.T) {
	blocks := []Block{{Label: "a"}, {Label: "b"}, {Label: "c"}}
	n := Tile(blocks)

	if got := labels(n); len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("labels in order = %v", got)
	}
	s := n.Split
	if s.Direction != "horizontal" || s.Ratio != 1.0/3 {
		t.Fatalf("root split = %s %v", s.Direction, s.Ratio)
	}
	if s.After.Split.Direction != "vertical" {
		t.Fatalf("nested split should stack, got %s", s.After.Split.Direction)
	}
}

func TestTileSingleBlock(t *testing.T) {
	if n := Tile([]Block{{Label: "a"}}); n.Block == nil || n.Split != nil {
		t.Fatalf("want a lone block node, got %+v", n)
	}
}
