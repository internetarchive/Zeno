package models

// DedupeItems dedupes items from any level, keeping in priority a Completed item
func (i *Item) DedupeItems() error {
	if !i.IsSeed() {
		return ErrNotASeed
	}

	// Flatten the tree into a list
	nodes := flattenTree(i)

	// Dedupe the nodes based on their URL
	urls := make(map[string]*Item)
	for _, node := range nodes {
		if node == nil || node.parent == nil {
			continue
		}
		if existing, ok := urls[node.url.String()]; ok {
			if existing.status != ItemCompleted && !existing.IsSeed() && node.status == ItemCompleted { // Keep the completed item
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

	children := node.GetChildren()
	for i := range children {
		markCompleted(children[i])
	}

	if (len(node.GetChildren()) == 0 || allChildrenCompleted(node.GetChildren())) && (node.status == ItemGotChildren || node.status == ItemGotRedirected) {
		node.status = ItemCompleted
	}
}

// allChildrenCompletedOrSeen checks if all children are completed
func allChildrenCompleted(children []*Item) bool {
	for i := range children {
		if children[i] == nil {
			continue
		}
		if children[i].HasWork() {
			return false
		}
	}
	return true
}
