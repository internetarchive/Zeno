package models

import (
	"errors"
	"testing"
)

func createTestItem(id string, seed bool, parent *Item) *Item {
	item := &Item{
		id:     id,
		seed:   seed,
		parent: parent,
	}
	if parent != nil {
		parent.children = append(parent.children, item)
	}
	return item
}

func TestItem_GetID(t *testing.T) {
	item := createTestItem("testID", true, nil)
	if got := item.GetID(); got != "testID" {
		t.Errorf("GetID() = %v, want %v", got, "testID")
	}
}

func TestItem_GetShortID(t *testing.T) {
	item := createTestItem("testID12345", true, nil)
	if got := item.GetShortID(); got != "testI" {
		t.Errorf("GetShortID() = %v, want %v", got, "testI")
	}
}

func TestItem_GetURL(t *testing.T) {
	url := &URL{Raw: "http://example.com"}
	item := createTestItem("testID", true, nil)
	item.url = url
	if got := item.GetURL(); got != url {
		t.Errorf("GetURL() = %v, want %v", got, url)
	}
}

func TestItem_IsSeed(t *testing.T) {
	item := createTestItem("testID", true, nil)
	if got := item.IsSeed(); got != true {
		t.Errorf("IsSeed() = %v, want %v", got, true)
	}
}

func TestItem_IsChild(t *testing.T) {
	item := createTestItem("testID", false, nil)
	if got := item.IsChild(); got != true {
		t.Errorf("IsChild() = %v, want %v", got, true)
	}
}

func TestItem_GetSeedVia(t *testing.T) {
	item := createTestItem("testID", true, nil)
	item.seedVia = "seedViaTest"
	if got := item.GetSeedVia(); got != "seedViaTest" {
		t.Errorf("GetSeedVia() = %v, want %v", got, "seedViaTest")
	}
}

func TestItem_GetStatus(t *testing.T) {
	status := ItemPostProcessed
	item := createTestItem("testID", true, nil)
	item.status = status
	if got := item.GetStatus(); got != status {
		t.Errorf("GetStatus() = %v, want %v", got, status)
	}
}

func TestItem_GetSource(t *testing.T) {
	source := ItemSourceHQ
	item := createTestItem("testID", true, nil)
	item.source = source
	if got := item.GetSource(); got != source {
		t.Errorf("GetSource() = %v, want %v", got, source)
	}
}

func TestItem_GetChildren(t *testing.T) {
	item := createTestItem("testID", true, nil)
	child := createTestItem("childID", false, item)
	if got := item.GetChildren(); len(got) != 1 || got[0] != child {
		t.Errorf("GetChildren() = %v, want %v", got, []*Item{child})
	}
}

func TestItem_GetParent(t *testing.T) {
	parent := createTestItem("parentID", true, nil)
	item := createTestItem("testID", false, parent)
	if got := item.GetParent(); got != parent {
		t.Errorf("GetParent() = %v, want %v", got, parent)
	}
}

func TestItem_GetError(t *testing.T) {
	err := errors.New("test error")
	item := createTestItem("testID", true, nil)
	item.err = err
	if got := item.GetError(); got != err {
		t.Errorf("GetError() = %v, want %v", got, err)
	}
}

func TestItem_CheckConsistency(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected error
	}{
		{
			name: "Valid seed item",
			item: func() *Item {
				item := createTestItem("testID", true, nil)
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: nil,
		},
		{
			name: "Valid child item",
			item: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.url = &URL{Raw: "http://example.com"}
				item := createTestItem("testID", false, parent)
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: nil,
		},
		{
			name:     "Item with nil URL",
			item:     createTestItem("testID", true, nil),
			expected: errors.New("url is nil"),
		},
		{
			name: "Item with empty ID",
			item: func() *Item {
				item := createTestItem("", true, nil)
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: errors.New("id is empty"),
		},
		{
			name: "Child item with seedVia",
			item: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.url = &URL{Raw: "http://example.com"}
				item := createTestItem("testID", false, parent)
				item.url = &URL{Raw: "http://example.com"}
				item.seedVia = "seedViaTest"
				return item
			}(),
			expected: errors.New("item is a child but has a seedVia"),
		},
		{
			name: "Seed item with parent",
			item: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.url = &URL{Raw: "http://example.com"}
				item := createTestItem("testID", true, parent)
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: errors.New("item is a seed but has a parent"),
		},
		{
			name: "Child item with no parent",
			item: func() *Item {
				item := createTestItem("testID", false, nil)
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: errors.New("item is a child but has no parent"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.CheckConsistency()
			if err != nil && err.Error() != tt.expected.Error() {
				t.Errorf("expected error: %v, got: %v", tt.expected, err)
			}
			if err == nil && tt.expected != nil {
				t.Errorf("expected error: %v, got: %v", tt.expected, err)
			}
		})
	}
}

func TestGetNodesAtLevel(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		root          *Item
		targetLevel   int
		expectedIDs   []string
		expectedError error
	}{
		{
			// root
			name:          "1-level seed",
			root:          createTestItem("root", true, nil),
			targetLevel:   0,
			expectedIDs:   []string{"root"},
			expectedError: nil,
		},
		{
			// root
			// ├── child1
			// └── child2
			name: "1-level tree",
			root: func() *Item {
				root := createTestItem("root", true, nil)
				createTestItem("child1", false, root)
				createTestItem("child2", false, root)
				return root
			}(),
			targetLevel:   1,
			expectedIDs:   []string{"child1", "child2"},
			expectedError: nil,
		},
		{
			// root
			// ├── child1
			// │   └── grandchild1
			// └── child2
			name: "3-level tree",
			root: func() *Item {
				root := createTestItem("root", true, nil)
				child1 := createTestItem("child1", false, root)
				createTestItem("child2", false, root)
				createTestItem("grandchild1", false, child1)
				return root
			}(),
			targetLevel:   2,
			expectedIDs:   []string{"grandchild1"},
			expectedError: nil,
		},
		{
			// root
			// ├── child1
			// │   └── grandchild1
			// └── child2
			name: "3-level tree with no Greatgrandchildren",
			root: func() *Item {
				root := createTestItem("root", true, nil)
				child1 := createTestItem("child1", false, root)
				createTestItem("child2", false, root)
				createTestItem("grandchild1", false, child1)
				return root
			}(),
			targetLevel:   3,
			expectedIDs:   nil,
			expectedError: nil,
		},
		{
			// root
			// ├── child1
			// │   ├── grandchild1
			// │   └── grandchild2
			// └── child2
			name: "3-level tree with 2 grandchildren at desired level",
			root: func() *Item {
				root := createTestItem("root", true, nil)
				child1 := createTestItem("child1", false, root)
				createTestItem("child2", false, root)
				createTestItem("grandchild1", false, child1)
				createTestItem("grandchild2", false, child1)
				return root
			}(),
			targetLevel:   2,
			expectedIDs:   []string{"grandchild1", "grandchild2"},
			expectedError: nil,
		},
		{
			// root
			// ├── child1
			// │   └── grandchild1
			// ├── child2
			// │   └── grandchild2
			// └── child3
			name: "3-level tree with 2 grandchildren from different parent at desired level",
			root: func() *Item {
				root := createTestItem("root", true, nil)
				child1 := createTestItem("child1", false, root)
				child2 := createTestItem("child2", false, root)
				createTestItem("child3", false, root)
				createTestItem("grandchild1", false, child1)
				createTestItem("grandchild2", false, child2)
				return root
			}(),
			targetLevel:   2,
			expectedIDs:   []string{"grandchild1", "grandchild2"},
			expectedError: nil,
		},
		{
			// root
			// ├── child1
			// │   ├── grandchild1
			// │   │   └── greatgrandchild1
			// │   └── grandchild2
			// └── child2
			//     └── grandchild3
			name: "4-level tree with 3 Grandchild with 1 Greatgrandchildren at desired level",
			root: func() *Item {
				root := createTestItem("root", true, nil)
				child1 := createTestItem("child1", false, root)
				child2 := createTestItem("child2", false, root)
				grandchild1 := createTestItem("grandchild1", false, child1)
				createTestItem("grandchild2", false, child1)
				createTestItem("grandchild3", false, child2)
				createTestItem("greatgrandchild1", false, grandchild1)
				return root
			}(),
			targetLevel:   3,
			expectedIDs:   []string{"greatgrandchild1"},
			expectedError: nil,
		},
		{
			// root
			// ├── child1
			// │   ├── grandchild1
			// │   │   ├── greatgrandchild1
			// │   │   │   ├── greatgreatgrandchild1
			// │   │   │   └── greatgreatgrandchild2
			// │   │   └── greatgrandchild2
			// │   └── grandchild2
			// │       └── greatgrandchild3
			// │           └── greatgreatgrandchild3
			// ├── child2
			// │   ├── grandchild3
			// │   │   └── greatgrandchild4
			// │   └── grandchild4
			// │       ├── greatgrandchild5
			// │       └── greatgrandchild6
			// └── child3
			//     ├── grandchild5
			//     └── grandchild6
			//         ├── greatgrandchild7
			//         │   └── greatgreatgrandchild4
			//         └── greatgrandchild8
			name: "5-level Ultra Complex Tree",
			root: func() *Item {
				root := createTestItem("root", true, nil)
				child1 := createTestItem("child1", false, root)
				child2 := createTestItem("child2", false, root)
				child3 := createTestItem("child3", false, root)

				grandchild1 := createTestItem("grandchild1", false, child1)
				grandchild2 := createTestItem("grandchild2", false, child1)
				grandchild3 := createTestItem("grandchild3", false, child2)
				grandchild4 := createTestItem("grandchild4", false, child2)
				createTestItem("grandchild5", false, child3)
				grandchild6 := createTestItem("grandchild6", false, child3)

				greatgrandchild1 := createTestItem("greatgrandchild1", false, grandchild1)
				createTestItem("greatgrandchild2", false, grandchild1)
				greatgrandchild3 := createTestItem("greatgrandchild3", false, grandchild2)
				createTestItem("greatgrandchild4", false, grandchild3)
				createTestItem("greatgrandchild5", false, grandchild4)
				createTestItem("greatgrandchild6", false, grandchild4)
				greatgrandchild7 := createTestItem("greatgrandchild7", false, grandchild6)
				createTestItem("greatgrandchild8", false, grandchild6)

				createTestItem("greatgreatgrandchild1", false, greatgrandchild1)
				createTestItem("greatgreatgrandchild2", false, greatgrandchild1)
				createTestItem("greatgreatgrandchild3", false, greatgrandchild3)
				createTestItem("greatgreatgrandchild4", false, greatgrandchild7)

				return root
			}(),
			targetLevel:   4,
			expectedIDs:   []string{"greatgreatgrandchild1", "greatgreatgrandchild2", "greatgreatgrandchild3", "greatgreatgrandchild4"},
			expectedError: nil,
		},
		{
			name:          "Non-seed item",
			root:          createTestItem("child", false, nil),
			targetLevel:   1,
			expectedIDs:   nil,
			expectedError: ErrNotASeed,
		},
		{
			name:          "Level not present",
			root:          createTestItem("root", true, nil),
			targetLevel:   1,
			expectedIDs:   nil,
			expectedError: nil,
		},
		{
			name: "Nil node",
			root: func() *Item {
				root := createTestItem("root", true, nil)
				child := createTestItem("child", false, root)
				child.children = append(child.children, nil) // Adding a nil child
				return root
			}(),
			targetLevel:   2,
			expectedIDs:   nil,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := tc.root.GetNodesAtLevel(tc.targetLevel)
			if err != tc.expectedError {
				t.Fatalf("Expected error %v, got %v", tc.expectedError, err)
			}

			var nodeIDs []string
			for _, node := range nodes {
				nodeIDs = append(nodeIDs, node.id)
			}

			if len(nodeIDs) != len(tc.expectedIDs) {
				t.Fatalf("Expected %d nodes, got %d", len(tc.expectedIDs), len(nodeIDs))
			}

			for i, id := range tc.expectedIDs {
				if nodeIDs[i] != id {
					t.Fatalf("Expected node ID %s, got %s", id, nodeIDs[i])
				}
			}
		})
	}
}

func TestGetSeed(t *testing.T) {
	// Test cases
	testCases := []struct {
		name        string
		item        *Item
		expectedID  string
		expectedErr error
	}{
		{
			name:        "Seed item",
			item:        createTestItem("root", true, nil),
			expectedID:  "root",
			expectedErr: nil,
		},
		{
			name: "Child item with seed parent",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				child := createTestItem("child", false, root)
				return child
			}(),
			expectedID:  "root",
			expectedErr: nil,
		},
		{
			name: "Grandchild item with seed grandparent",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				child := createTestItem("child", false, root)
				grandchild := createTestItem("grandchild", false, child)
				return grandchild
			}(),
			expectedID:  "root",
			expectedErr: nil,
		},
		{
			name:        "Non-seed item with no parent",
			item:        createTestItem("child", false, nil),
			expectedID:  "",
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seed := tc.item.GetSeed()
			if seed == nil && tc.expectedID != "" {
				t.Fatalf("Expected seed ID %s, got nil", tc.expectedID)
			}
			if seed != nil && seed.id != tc.expectedID {
				t.Fatalf("Expected seed ID %s, got %s", tc.expectedID, seed.id)
			}
		})
	}
}

func TestItem_GetDepth(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected int64
	}{
		{
			name:     "Seed item",
			item:     createTestItem("root", true, nil),
			expected: 0,
		},
		{
			name: "Child item with depth 1",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				child := createTestItem("child", false, root)
				return child
			}(),
			expected: 1,
		},
		{
			name: "Grandchild item with depth 2",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				child := createTestItem("child", false, root)
				grandchild := createTestItem("grandchild", false, child)
				return grandchild
			}(),
			expected: 2,
		},
		{
			name: "Great-grandchild item with depth 3",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				child := createTestItem("child", false, root)
				grandchild := createTestItem("grandchild", false, child)
				greatGrandchild := createTestItem("greatGrandchild", false, grandchild)
				return greatGrandchild
			}(),
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.GetDepth(); got != tt.expected {
				t.Errorf("GetDepth() = %v, want %v", got, tt.expected)
			}
		})
	}
}
func TestItem_GetMaxDepth(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected int64
	}{
		{
			name:     "Single seed item",
			item:     createTestItem("root", true, nil),
			expected: 0,
		},
		{
			name: "Seed with one child",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				createTestItem("child", false, root)
				return root
			}(),
			expected: 1,
		},
		{
			name: "Seed with two levels of children",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				child := createTestItem("child", false, root)
				createTestItem("grandchild", false, child)
				return root
			}(),
			expected: 2,
		},
		{
			name: "Seed with multiple children at different levels",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				child1 := createTestItem("child1", false, root)
				child2 := createTestItem("child2", false, root)
				createTestItem("grandchild1", false, child1)
				createTestItem("grandchild2", false, child2)
				createTestItem("greatGrandchild", false, child1.children[0])
				return root
			}(),
			expected: 3,
		},
		{
			name:     "Seed with no children",
			item:     createTestItem("root", true, nil),
			expected: 0,
		},
		{
			name: "Seed with multiple children at same level",
			item: func() *Item {
				root := createTestItem("root", true, nil)
				createTestItem("child1", false, root)
				createTestItem("child2", false, root)
				createTestItem("child3", false, root)
				return root
			}(),
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.GetMaxDepth(); got != tt.expected {
				t.Errorf("GetMaxDepth() = %v, want %v", got, tt.expected)
			}
		})
	}
}
