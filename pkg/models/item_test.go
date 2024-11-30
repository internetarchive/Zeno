package models

import (
	"errors"
	"fmt"
	"sync"
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
func TestAddChild(t *testing.T) {
	tests := []struct {
		name        string
		parent      *Item
		child       *Item
		from        ItemState
		expectedErr error
	}{
		{
			name: "Valid AddChild with ItemGotRedirected",
			parent: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.status = ItemGotRedirected
				return parent
			}(),
			child:       createTestItem("childID", false, nil),
			from:        ItemGotRedirected,
			expectedErr: nil,
		},
		{
			name: "Valid AddChild with ItemGotChildren",
			parent: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.status = ItemGotChildren
				return parent
			}(),
			child:       createTestItem("childID", false, nil),
			from:        ItemGotChildren,
			expectedErr: nil,
		},
		{
			name: "Invalid AddChild with wrong state",
			parent: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.status = ItemFresh
				return parent
			}(),
			child:       createTestItem("childID", false, nil),
			from:        ItemFresh,
			expectedErr: fmt.Errorf("from state is invalid, only ItemGotRedirected and ItemGotChildren are allowed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parent.AddChild(tt.child, tt.from)
			if err != nil && err.Error() != tt.expectedErr.Error() {
				t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if err == nil && tt.expectedErr != nil {
				t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if err == nil {
				if tt.child.parent != tt.parent {
					t.Errorf("expected parent: %v, got: %v", tt.parent, tt.child.parent)
				}
				if len(tt.parent.children) != 1 || tt.parent.children[0] != tt.child {
					t.Errorf("expected child: %v, got: %v", tt.child, tt.parent.children)
				}
			}
		})
	}
}
func TestItem_SetSource(t *testing.T) {
	tests := []struct {
		name        string
		item        *Item
		source      ItemSource
		expectedErr error
	}{
		{
			name:        "Set source for seed item to ItemSourceInsert",
			item:        createTestItem("testID", true, nil),
			source:      ItemSourceInsert,
			expectedErr: nil,
		},
		{
			name:        "Set source for seed item to ItemSourceQueue",
			item:        createTestItem("testID", true, nil),
			source:      ItemSourceQueue,
			expectedErr: nil,
		},
		{
			name:        "Set source for seed item to ItemSourceHQ",
			item:        createTestItem("testID", true, nil),
			source:      ItemSourceHQ,
			expectedErr: nil,
		},
		{
			name:        "Set source for seed item to ItemSourcePostprocess",
			item:        createTestItem("testID", true, nil),
			source:      ItemSourcePostprocess,
			expectedErr: nil,
		},
		{
			name:        "Set source for seed item to ItemSourceFeedback",
			item:        createTestItem("testID", true, nil),
			source:      ItemSourceFeedback,
			expectedErr: nil,
		},
		{
			name:        "Set source for child item to ItemSourceInsert",
			item:        createTestItem("testID", false, createTestItem("parentID", true, nil)),
			source:      ItemSourceInsert,
			expectedErr: fmt.Errorf("source is invalid for a child"),
		},
		{
			name:        "Set source for child item to ItemSourceQueue",
			item:        createTestItem("testID", false, createTestItem("parentID", true, nil)),
			source:      ItemSourceQueue,
			expectedErr: fmt.Errorf("source is invalid for a child"),
		},
		{
			name:        "Set source for child item to ItemSourceHQ",
			item:        createTestItem("testID", false, createTestItem("parentID", true, nil)),
			source:      ItemSourceHQ,
			expectedErr: fmt.Errorf("source is invalid for a child"),
		},
		{
			name:        "Set source for child item to ItemSourcePostprocess",
			item:        createTestItem("testID", false, createTestItem("parentID", true, nil)),
			source:      ItemSourcePostprocess,
			expectedErr: nil,
		},
		{
			name:        "Set source for child item to ItemSourceFeedback",
			item:        createTestItem("testID", false, createTestItem("parentID", true, nil)),
			source:      ItemSourceFeedback,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.SetSource(tt.source)
			if err != nil && err.Error() != tt.expectedErr.Error() {
				t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if err == nil && tt.expectedErr != nil {
				t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if err == nil && tt.item.source != tt.source {
				t.Errorf("expected source: %v, got: %v", tt.source, tt.item.source)
			}
		})
	}
}

func TestItem_SetStatus(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		status   ItemState
		expected ItemState
	}{
		{
			name:     "Set status to ItemFresh",
			item:     createTestItem("testID", true, nil),
			status:   ItemFresh,
			expected: ItemFresh,
		},
		{
			name:     "Set status to ItemPreProcessed",
			item:     createTestItem("testID", true, nil),
			status:   ItemPreProcessed,
			expected: ItemPreProcessed,
		},
		{
			name:     "Set status to ItemArchived",
			item:     createTestItem("testID", true, nil),
			status:   ItemArchived,
			expected: ItemArchived,
		},
		{
			name:     "Set status to ItemPostProcessed",
			item:     createTestItem("testID", true, nil),
			status:   ItemPostProcessed,
			expected: ItemPostProcessed,
		},
		{
			name:     "Set status to ItemFailed",
			item:     createTestItem("testID", true, nil),
			status:   ItemFailed,
			expected: ItemFailed,
		},
		{
			name:     "Set status to ItemCompleted",
			item:     createTestItem("testID", true, nil),
			status:   ItemCompleted,
			expected: ItemCompleted,
		},
		{
			name:     "Set status to ItemGotRedirected",
			item:     createTestItem("testID", true, nil),
			status:   ItemGotRedirected,
			expected: ItemGotRedirected,
		},
		{
			name:     "Set status to ItemGotChildren",
			item:     createTestItem("testID", true, nil),
			status:   ItemGotChildren,
			expected: ItemGotChildren,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.item.SetStatus(tt.status)
			if got := tt.item.GetStatus(); got != tt.expected {
				t.Errorf("SetStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestItem_SetError(t *testing.T) {
	tests := []struct {
		name        string
		item        *Item
		err         error
		expectedErr error
	}{
		{
			name:        "Set error to nil",
			item:        createTestItem("testID", true, nil),
			err:         nil,
			expectedErr: nil,
		},
		{
			name:        "Set error to non-nil error",
			item:        createTestItem("testID", true, nil),
			err:         errors.New("test error"),
			expectedErr: errors.New("test error"),
		},
		{
			name:        "Set error to another non-nil error",
			item:        createTestItem("testID", true, nil),
			err:         errors.New("another test error"),
			expectedErr: errors.New("another test error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.item.SetError(tt.err)
			if got := tt.item.GetError(); got != nil && got.Error() != tt.expectedErr.Error() {
				t.Errorf("SetError() = %v, want %v", got, tt.expectedErr)
			}
			if got := tt.item.GetError(); got == nil && tt.expectedErr != nil {
				t.Errorf("SetError() = %v, want %v", got, tt.expectedErr)
			}
		})
	}
}

func TestNewItem(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		url      *URL
		via      string
		isSeed   bool
		expected *Item
	}{
		{
			name:   "Create seed item",
			id:     "testID",
			url:    &URL{Raw: "http://example.com"},
			via:    "seedViaTest",
			isSeed: true,
			expected: &Item{
				id:      "testID",
				url:     &URL{Raw: "http://example.com"},
				seed:    true,
				seedVia: "seedViaTest",
				status:  ItemFresh,
			},
		},
		{
			name:   "Create child item",
			id:     "childID",
			url:    &URL{Raw: "http://example.com/child"},
			via:    "",
			isSeed: false,
			expected: &Item{
				id:      "childID",
				url:     &URL{Raw: "http://example.com/child"},
				seed:    false,
				seedVia: "",
				status:  ItemFresh,
			},
		},
		{
			name:   "Create seed item with empty seedVia",
			id:     "testID2",
			url:    &URL{Raw: "http://example.com/2"},
			via:    "",
			isSeed: true,
			expected: &Item{
				id:      "testID2",
				url:     &URL{Raw: "http://example.com/2"},
				seed:    true,
				seedVia: "",
				status:  ItemFresh,
			},
		},
		{
			name:   "Create child item with non-empty seedVia",
			id:     "childID2",
			url:    &URL{Raw: "http://example.com/child2"},
			via:    "seedViaTest2",
			isSeed: false,
			expected: &Item{
				id:      "childID2",
				url:     &URL{Raw: "http://example.com/child2"},
				seed:    false,
				seedVia: "seedViaTest2",
				status:  ItemFresh,
			},
		},
		{
			name:     "Create seed item with nil URL",
			id:       "testID3",
			url:      nil,
			via:      "",
			isSeed:   true,
			expected: nil,
		},
		{
			name:     "Create child item with empty ID",
			id:       "",
			url:      &URL{Raw: "http://example.com/child3"},
			via:      "",
			isSeed:   false,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := NewItem(tt.id, tt.url, tt.via, tt.isSeed)
			if tt.expected == nil && item != nil {
				t.Errorf("expected nil, got: %v", item)
			} else if item != nil {
				if item.id != tt.expected.id {
					t.Errorf("expected id: %v, got: %v", tt.expected.id, item.id)
				}
				if item.url.Raw != tt.expected.url.Raw {
					t.Errorf("expected url: %v, got: %v", tt.expected.url.Raw, item.url.Raw)
				}
				if item.seed != tt.expected.seed {
					t.Errorf("expected seed: %v, got: %v", tt.expected.seed, item.seed)
				}
				if item.seedVia != tt.expected.seedVia {
					t.Errorf("expected seedVia: %v, got: %v", tt.expected.seedVia, item.seedVia)
				}
				if item.status != tt.expected.status {
					t.Errorf("expected status: %v, got: %v", tt.expected.status, item.status)
				}
			}
		})
	}
}

func TestItem_IsRedirection(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected bool
	}{
		{
			name: "Item with parent having status ItemGotRedirected",
			item: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.status = ItemGotRedirected
				child := createTestItem("childID", false, parent)
				return child
			}(),
			expected: true,
		},
		{
			name: "Item with parent having status ItemGotChildren",
			item: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.status = ItemGotChildren
				child := createTestItem("childID", false, parent)
				return child
			}(),
			expected: false,
		},
		{
			name:     "Item with no parent",
			item:     createTestItem("testID", true, nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.IsRedirection(); got != tt.expected {
				t.Errorf("IsRedirection() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestItem_IsAChild(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected bool
	}{
		{
			name: "Item with parent having status ItemGotChildren",
			item: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.status = ItemGotChildren
				child := createTestItem("childID", false, parent)
				return child
			}(),
			expected: true,
		},
		{
			name: "Item with parent having status ItemGotRedirected",
			item: func() *Item {
				parent := createTestItem("parentID", true, nil)
				parent.status = ItemGotRedirected
				child := createTestItem("childID", false, parent)
				return child
			}(),
			expected: false,
		},
		{
			name:     "Item with no parent",
			item:     createTestItem("testID", true, nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.IsAChild(); got != tt.expected {
				t.Errorf("IsAChild() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConcurrentAddChild(t *testing.T) {
	parent := createTestItem("parentID", true, nil)
	parent.status = ItemGotChildren

	var wg sync.WaitGroup
	numChildren := 100
	wg.Add(numChildren)

	for i := 0; i < numChildren; i++ {
		go func(i int) {
			defer wg.Done()
			child := createTestItem(fmt.Sprintf("childID%d", i), false, nil)
			err := parent.AddChild(child, ItemGotChildren)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}

	wg.Wait()

	if len(parent.children) != numChildren {
		t.Errorf("expected %d children, got %d", numChildren, len(parent.children))
	}
}