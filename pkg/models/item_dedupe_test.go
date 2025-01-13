package models

import "testing"

func TestItem_DedupeChilds(t *testing.T) {
	tests := []struct {
		name           string
		setupTree      func() *Item
		expectedIDs    map[string][]string
		expectedStatus map[string]ItemState
	}{
		{
			name: "No duplicates",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.SetStatus(ItemGotChildren)
				createTestItemWithURL("child1", false, root, "http://example.com/child1")
				createTestItemWithURL("child2", false, root, "http://example.com/child2")
				return root
			},
			expectedIDs: map[string][]string{
				"root": {"child1", "child2"},
			},
			expectedStatus: map[string]ItemState{
				"root": ItemGotChildren,
			},
		},
		{
			name: "With duplicates",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.SetStatus(ItemGotChildren)
				createTestItemWithURL("child1", false, root, "http://example.com/child")
				createTestItemWithURL("child2", false, root, "http://example.com/child")
				return root
			},
			expectedIDs: map[string][]string{
				"root": {"child1"},
			},
			expectedStatus: map[string]ItemState{
				"root": ItemGotChildren,
			},
		},
		{
			name: "Multiple duplicates",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.SetStatus(ItemGotChildren)
				createTestItemWithURL("child1", false, root, "http://example.com/child")
				createTestItemWithURL("child2", false, root, "http://example.com/child")
				createTestItemWithURL("child3", false, root, "http://example.com/child")
				return root
			},
			expectedIDs: map[string][]string{
				"root": {"child1"},
			},
			expectedStatus: map[string]ItemState{
				"root": ItemGotChildren,
			},
		},
		{
			name: "Complex tree with duplicates",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.SetStatus(ItemGotChildren)
				child1 := createTestItemWithURL("child1", false, root, "http://example.com/child1")
				child1.SetStatus(ItemGotChildren)
				child2 := createTestItemWithURL("child2", false, root, "http://example.com/child2")
				child2.SetStatus(ItemGotChildren)
				createTestItemWithURL("grandchild1", false, child1, "http://example.com/grandchild")
				createTestItemWithURL("grandchild2", false, child1, "http://example.com/grandchild")
				createTestItemWithURL("grandchild3", false, child2, "http://example.com/grandchild")
				return root
			},
			expectedIDs: map[string][]string{
				"root":   {"child1", "child2"},
				"child1": {"grandchild1"},
				"child2": {},
			},
			expectedStatus: map[string]ItemState{
				"root":   ItemGotChildren,
				"child1": ItemGotChildren,
				"child2": ItemCompleted,
			},
		},
		{
			name: "Complex tree with multi-level duplicates",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.SetStatus(ItemGotChildren)
				child1 := createTestItemWithURL("child1", false, root, "http://example.com/child1")
				child1.SetStatus(ItemGotChildren)
				child2 := createTestItemWithURL("child2", false, root, "http://example.com/this-was-crawled")
				child2.SetStatus(ItemCompleted)
				createTestItemWithURL("grandchild1", false, child1, "http://example.com/this-was-crawled")
				return root
			},
			expectedIDs: map[string][]string{
				"root":   {"child1", "child2"},
				"child1": {},
				"child2": {},
			},
			expectedStatus: map[string]ItemState{
				"root":   ItemCompleted,
				"child1": ItemCompleted,
				"child2": ItemCompleted,
			},
		},
		{
			name: "Valid seed item with no duplicates",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.SetStatus(ItemGotChildren)
				createTestItemWithURL("child1", false, root, "http://example.com/child1")
				createTestItemWithURL("child2", false, root, "http://example.com/child2")
				return root
			},
			expectedIDs: map[string][]string{
				"root": {"child1", "child2"},
			},
			expectedStatus: map[string]ItemState{
				"root": ItemGotChildren,
			},
		},
		{
			name: "Valid seed item with duplicates",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.SetStatus(ItemGotChildren)
				createTestItemWithURL("child1", false, root, "http://example.com/child")
				createTestItemWithURL("child2", false, root, "http://example.com/child")
				return root
			},
			expectedIDs: map[string][]string{
				"root": {"child1"},
			},
			expectedStatus: map[string]ItemState{
				"root": ItemGotChildren,
			},
		},
		{
			name: "Item with completed duplicate",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.SetStatus(ItemGotChildren)
				createTestItemWithURL("child1", false, root, "http://example.com/child")
				child2 := createTestItemWithURL("child2", false, root, "http://example.com/child")
				child2.status = ItemCompleted
				return root
			},
			expectedIDs: map[string][]string{
				"root": {"child2"},
			},
			expectedStatus: map[string]ItemState{
				"root":   ItemCompleted,
				"child2": ItemCompleted,
			},
		},
		{
			name: "Item with nil child",
			setupTree: func() *Item {
				root := createTestItemWithURL("root", true, nil, "http://example.com/root")
				root.children = append(root.children, nil)
				return root
			},
			expectedIDs: map[string][]string{
				"root": {},
			},
			expectedStatus: map[string]ItemState{
				"root": ItemFresh,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setupTree()
			root.DedupeItems()

			// Check if the deduped items are correct
			for parentID, expectedChildrenIDs := range tt.expectedIDs {
				var parent *Item
				if parentID == "root" {
					parent = root
				} else {
					parent = findTestItemByID(root, parentID)
				}
				if parent == nil {
					t.Fatalf("parent with id %s not found", parentID)
				}

				children := parent.GetChildren()
				if len(children) != len(expectedChildrenIDs) {
					t.Fatalf("expected %d children for parent %s, got %d", len(expectedChildrenIDs), parentID, len(children))
				}

				expectedIDsMap := make(map[string]bool)
				for _, id := range expectedChildrenIDs {
					expectedIDsMap[id] = true
				}

				for _, child := range children {
					if !expectedIDsMap[child.id] {
						t.Fatalf("unexpected child: %s for parent %s", child.id, parentID)
					}
				}
			}

			// Check if the statuses are correct
			for id, expectedStatus := range tt.expectedStatus {
				var item *Item
				if id == "root" {
					item = root
				} else {
					item = findTestItemByID(root, id)
				}
				if item == nil {
					t.Fatalf("item with id %s not found", id)
				}
				if item.GetStatus() != expectedStatus {
					t.Fatalf("expected status %v for item %s, got %v", expectedStatus, id, item.GetStatus())
				}
			}
		})
	}
}
