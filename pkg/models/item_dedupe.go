package models

// DedupeItems dedupes items from any level, keeping in priority a Completed item
func (i *Item) DedupeItems() error {
	if !i.seed {
		return ErrNotASeed
	}

	// Flatten the tree into a list
	nodes := flattenTree(i)

	// Dedupe the nodes based on their URL
	urls := make(map[string]*Item)
	for _, node := range nodes {
		if node == nil || (!node.seed && node.parent == nil) {
			continue
		}
		if existing, ok := urls[node.url.String()]; ok {
			if existing.status != ItemCompleted && node.status == ItemCompleted { // Keep the completed item
				existing.parent.RemoveChild(existing)
				urls[node.url.String()] = node
			} else {
				node.parent.RemoveChild(node)
			}
		} else {
			urls[node.url.String()] = node
		}
	}

	// Traverse the tree to mark items as completed
	markCompleted(i)

	return nil
}

// flattenTree flattens the tree into a list of nodes
func flattenTree(root *Item) []*Item {
	var nodes []*Item
	var traverse func(node *Item)
	traverse = func(node *Item) {
		nodes = append(nodes, node)
		if node == nil {
			return
		}
		for _, child := range node.GetChildren() {
			traverse(child)
		}
	}
	traverse(root)
	return nodes
}

// markCompleted marks items as completed if they have no children or all their children are completed
func markCompleted(node *Item) {
	if node == nil {
		return
	}

	for _, child := range node.GetChildren() {
		markCompleted(child)
	}

	if (len(node.GetChildren()) == 0 || allChildrenCompleted(node.GetChildren())) && node.status == ItemGotChildren {
		node.status = ItemCompleted
	}
}

// allChildrenCompleted checks if all children are completed
func allChildrenCompleted(children []*Item) bool {
	for _, child := range children {
		if child == nil {
			continue
		}
		if child.status != ItemCompleted {
			return false
		}
	}
	return true
}
