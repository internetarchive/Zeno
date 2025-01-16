package models

import "strings"

// DrawTree generates the ASCII representation of the tree
func (i *Item) DrawTree() string {
	topmostParent := i.GetSeed()
	if topmostParent == nil {
		return ""
	}
	var sb strings.Builder
	topmostParent.draw(&sb, "", true, true)
	return sb.String()
}

// DrawTree generates the ASCII representation of the tree
func (i *Item) DrawTreeWithStatus() string {
	topmostParent := i.GetSeed()
	if topmostParent == nil {
		return ""
	}
	var sb strings.Builder
	topmostParent.drawWithStatus(&sb, "", true, true)
	return sb.String()
}

// draw is a helper function to recursively build the ASCII tree
func (i *Item) draw(sb *strings.Builder, prefix string, isTail bool, isRoot bool) {
	if !isRoot {
		sb.WriteString(prefix)
		if isTail {
			sb.WriteString("└── ")
			prefix += "    "
		} else {
			sb.WriteString("├── ")
			prefix += "│   "
		}
	}
	sb.WriteString(i.id + "\n")

	for idx, child := range i.children {
		child.draw(sb, prefix, idx == len(i.children)-1, false)
	}
}

// draw is a helper function to recursively build the ASCII tree
func (i *Item) drawWithStatus(sb *strings.Builder, prefix string, isTail bool, isRoot bool) {
	if !isRoot {
		sb.WriteString(prefix)
		if isTail {
			sb.WriteString("└── ")
			prefix += "    "
		} else {
			sb.WriteString("├── ")
			prefix += "│   "
		}
	}
	sb.WriteString(i.id + " - " + i.GetStatus().String() + "\n")

	for idx, child := range i.children {
		child.draw(sb, prefix, idx == len(i.children)-1, false)
	}
}
