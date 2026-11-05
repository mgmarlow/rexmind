// Package layout builds Rex window layout trees.
package layout

// Node is one node of a Rex layout: either a block or a split.
type Node struct {
	Block *Block `json:"block,omitempty"`
	Split *Split `json:"split,omitempty"`
}

// Block starts one new block.
type Block struct {
	Flavor  string `json:"flavor"`
	Label   string `json:"label,omitempty"`
	Options any    `json:"options,omitempty"`
}

// Split divides its space between Before and After.
type Split struct {
	Direction string  `json:"direction"` // horizontal (side by side) or vertical (stacked)
	Ratio     float64 `json:"ratio"`
	Before    Node    `json:"before"`
	After     Node    `json:"after"`
}

// Tile arranges blocks in a balanced grid: halves alternate between side by
// side and stacked, and each split's ratio keeps every block the same size.
func Tile(blocks []Block) Node {
	return tile(blocks, "horizontal")
}

func tile(blocks []Block, dir string) Node {
	if len(blocks) == 1 {
		b := blocks[0]
		return Node{Block: &b}
	}
	mid := len(blocks) / 2
	return Node{Split: &Split{
		Direction: dir,
		Ratio:     float64(mid) / float64(len(blocks)),
		Before:    tile(blocks[:mid], flip(dir)),
		After:     tile(blocks[mid:], flip(dir)),
	}}
}

func flip(dir string) string {
	if dir == "horizontal" {
		return "vertical"
	}
	return "horizontal"
}
