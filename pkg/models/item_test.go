package models

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func createTestItem(id string, parent *Item) *Item {
	item := &Item{
		id:       id,
		parent:   parent,
		status:   ItemFresh,
		children: make([]*Item, 0),
	}
	if parent != nil {
		parent.children = append(parent.children, item)
	}
	return item
}

func createTestItemWithURL(id string, parent *Item, url string) *Item {
	item := createTestItem(id, parent)
	newURL, err := NewURL(url)
	if err != nil {
		panic(err)
	}
	item.url = &newURL
	return item
}

func createTestItemWithStatus(id string, parent *Item, status ItemState) *Item {
	item := &Item{
		id:     id,
		parent: parent,
		status: status,
	}
	if parent != nil {
		parent.children = append(parent.children, item)
	}
	return item
}

func findTestItemByID(root *Item, id string) *Item {
	if root.id == id {
		return root
	}
	for _, child := range root.GetChildren() {
		if found := findTestItemByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func TestItem_GetID(t *testing.T) {
	item := createTestItem("testID", nil)
	if got := item.GetID(); got != "testID" {
		t.Errorf("GetID() = %v, want %v", got, "testID")
	}
}

func TestItem_GetShortID(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected string
	}{
		{
			name: "Simple short ID",
			item: func() *Item {
				item := createTestItem("short", nil)
				return item
			}(),
			expected: "short",
		},
		{
			name: "UUID ID",
			item: func() *Item {
				item := createTestItem("f47ac10b-58cc-4372-a567-0e02b2c3d479", nil)
				return item
			}(),
			expected: "f47ac",
		},
		{
			name: "SHA1 ID",
			item: func() *Item {
				item := createTestItem("f47ac10b58cc4372a5670e02b2c3d479", nil)
				return item
			}(),
			expected: "f47ac",
		},
		{
			name: "HQ seed ID",
			item: func() *Item {
				item := createTestItem("seed-bc3446c00ff1bbcc167aa611613264e1adced419bdafe08adbca35f4566297a1", nil)
				return item
			}(),
			expected: "seed-bc344",
		},
		{
			name: "HQ asset ID",
			item: func() *Item {
				item := createTestItem("asset-bc3446c00ff1bbcc167aa611613264e1adced419bdafe08adbca35f4566297a1", nil)
				return item
			}(),
			expected: "asset-bc344",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortID := tt.item.GetShortID()
			if tt.expected != shortID {
				t.Errorf("expected short ID: %s, got: %s", tt.expected, shortID)
			}
		})
	}
}

func TestItem_GetURL(t *testing.T) {
	url := &URL{Raw: "http://example.com"}
	item := createTestItem("testID", nil)
	item.url = url
	if got := item.GetURL(); got != url {
		t.Errorf("GetURL() = %v, want %v", got, url)
	}
}

func TestItem_IsSeed(t *testing.T) {
	item := createTestItem("testID", nil)
	if got := item.IsSeed(); got != true {
		t.Errorf("IsSeed() = %v, want %v", got, true)
	}
}

func TestItem_GetSeedVia(t *testing.T) {
	item := createTestItem("testID", nil)
	item.seedVia = "seedViaTest"
	if got := item.GetSeedVia(); got != "seedViaTest" {
		t.Errorf("GetSeedVia() = %v, want %v", got, "seedViaTest")
	}
}

func TestItem_GetStatus(t *testing.T) {
	status := ItemArchived
	item := createTestItem("testID", nil)
	item.status = status
	if got := item.GetStatus(); got != status {
		t.Errorf("GetStatus() = %v, want %v", got, status)
	}
}

func TestItem_GetSource(t *testing.T) {
	source := ItemSourceHQ
	item := createTestItem("testID", nil)
	item.source = source
	if got := item.GetSource(); got != source {
		t.Errorf("GetSource() = %v, want %v", got, source)
	}
}

func TestItem_GetChildren(t *testing.T) {
	item := createTestItem("testID", nil)
	child := createTestItem("childID", item)
	if got := item.GetChildren(); len(got) != 1 || got[0] != child {
		t.Errorf("GetChildren() = %v, want %v", got, []*Item{child})
	}
}

func TestItem_GetParent(t *testing.T) {
	parent := createTestItem("parentID", nil)
	item := createTestItem("testID", parent)
	if got := item.GetParent(); got != parent {
		t.Errorf("GetParent() = %v, want %v", got, parent)
	}
}

func TestItem_GetError(t *testing.T) {
	err := errors.New("test error")
	item := createTestItem("testID", nil)
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
				item := createTestItem("testID", nil)
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: nil,
		},
		{
			name: "Valid child item",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				parent.url = &URL{Raw: "http://example.com"}
				item := createTestItem("testID", parent)
				item.url = &URL{Raw: "http://example.com"}
				item.parent.status = ItemGotChildren
				return item
			}(),
			expected: nil,
		},
		{
			name:     "Item with nil URL",
			item:     createTestItem("testID", nil),
			expected: errors.New("url is nil"),
		},
		{
			name: "Item with empty ID",
			item: func() *Item {
				item := createTestItem("", nil)
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: errors.New("id is empty"),
		},
		{
			name: "Child item with seedVia",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				parent.url = &URL{Raw: "http://example.com"}
				item := createTestItem("testID", parent)
				item.url = &URL{Raw: "http://example.com"}
				item.seedVia = "seedViaTest"
				return item
			}(),
			expected: errors.New("item is a child but has a seedVia"),
		},
		{
			name: "Item is fresh but has children",
			item: func() *Item {
				root := createTestItem("root", nil)
				root.url = &URL{Raw: "http://example.com/root"}
				root.status = ItemFresh
				createTestItem("child1", root)
				return root
			}(),
			expected: errors.New("item is fresh but has children"),
		},
		{
			name: "Item is fresh but parent is not ItemGotChildren or ItemGotRedirected",
			item: func() *Item {
				parent := createTestItem("parent", nil)
				parent.url = &URL{Raw: "http://example.com/parent"}
				parent.status = ItemFresh
				child := createTestItem("child", parent)
				child.url = &URL{Raw: "http://example.com/child"}
				child.status = ItemFresh
				return child
			}(),
			expected: errors.New("item is not a seed and fresh but parent is not ItemGotChildren or ItemGotRedirected"),
		},
		{
			name: "Item has more than one children but is ItemGotRedirected",
			item: func() *Item {
				root := createTestItem("root", nil)
				root.url = &URL{Raw: "http://example.com/root"}
				root.status = ItemGotRedirected
				createTestItem("child1", root)
				createTestItem("child2", root)
				return root
			}(),
			expected: errors.New("item has more than one children but is ItemGotRedirected"),
		},
		{
			name: "Item has children but is not ItemGotChildren, ItemGotRedirected, ItemCompleted or ItemFailed",
			item: func() *Item {
				root := createTestItem("root", nil)
				root.url = &URL{Raw: "http://example.com/root"}
				root.status = ItemArchived
				createTestItem("child1", root)
				return root
			}(),
			expected: errors.New("item has children but is not ItemGotChildren, ItemGotRedirected, ItemCompleted or ItemFailed"),
		},
		{
			name: "Valid seed item",
			item: func() *Item {
				item := createTestItem("testID", nil)
				item.status = ItemFresh
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: nil,
		},
		{
			name: "Valid child item",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				parent.status = ItemGotChildren
				parent.url = &URL{Raw: "http://example.com"}
				item := createTestItem("testID", parent)
				item.status = ItemFresh
				item.url = &URL{Raw: "http://example.com"}
				return item
			}(),
			expected: nil,
		},
		{
			name: "Valid item with children and status ItemGotChildren",
			item: func() *Item {
				root := createTestItem("root", nil)
				root.status = ItemGotChildren
				root.url = &URL{Raw: "http://example.com/root"}
				child := createTestItem("child1", root)
				child.url = &URL{Raw: "http://example.com/child"}
				child.status = ItemFresh
				return root
			}(),
			expected: nil,
		},
		{
			name: "Valid item with children and status ItemGotRedirected",
			item: func() *Item {
				root := createTestItem("root", nil)
				root.status = ItemGotRedirected
				root.url = &URL{Raw: "http://example.com/root"}
				child := createTestItem("child1", root)
				child.url = &URL{Raw: "http://example.com/child"}
				child.status = ItemFresh
				return root
			}(),
			expected: nil,
		},
		{
			name: "Valid item with fresh children and status ItemCompleted",
			item: func() *Item {
				root := createTestItem("root", nil)
				root.status = ItemCompleted
				root.url = &URL{Raw: "http://example.com/root"}
				child := createTestItem("child1", root)
				child.url = &URL{Raw: "http://example.com/child"}
				child.status = ItemFresh
				return root
			}(),
			expected: errors.New("child child1: item is not a seed and fresh but parent is not ItemGotChildren or ItemGotRedirected"),
		},
		{
			name: "Valid item with seen children and status ItemCompleted",
			item: func() *Item {
				root := createTestItem("root", nil)
				root.status = ItemCompleted
				root.url = &URL{Raw: "http://example.com/root"}
				child := createTestItem("child1", root)
				child.url = &URL{Raw: "http://example.com/child"}
				child.status = ItemSeen
				return root
			}(),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.CheckConsistency()
			if err != nil && tt.expected != nil && err.Error() != tt.expected.Error() {
				t.Errorf("expected error: %v, got: %v", tt.expected, err)
			}
			if err == nil && tt.expected != nil {
				t.Errorf("expected error: %v, got: %v", tt.expected, err)
			}
			if err != nil && tt.expected == nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestGetNodesAtLevel(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		root          *Item
		targetLevel   int64
		expectedIDs   []string
		expectedError error
	}{
		{
			// root
			name:          "1-level seed",
			root:          createTestItem("root", nil),
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
				root := createTestItem("root", nil)
				createTestItem("child1", root)
				createTestItem("child2", root)
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
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				createTestItem("child2", root)
				createTestItem("grandchild1", child1)
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
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				createTestItem("child2", root)
				createTestItem("grandchild1", child1)
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
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				createTestItem("child2", root)
				createTestItem("grandchild1", child1)
				createTestItem("grandchild2", child1)
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
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				child2 := createTestItem("child2", root)
				createTestItem("child3", root)
				createTestItem("grandchild1", child1)
				createTestItem("grandchild2", child2)
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
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				child2 := createTestItem("child2", root)
				grandchild1 := createTestItem("grandchild1", child1)
				createTestItem("grandchild2", child1)
				createTestItem("grandchild3", child2)
				createTestItem("greatgrandchild1", grandchild1)
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
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				child2 := createTestItem("child2", root)
				child3 := createTestItem("child3", root)

				grandchild1 := createTestItem("grandchild1", child1)
				grandchild2 := createTestItem("grandchild2", child1)
				grandchild3 := createTestItem("grandchild3", child2)
				grandchild4 := createTestItem("grandchild4", child2)
				createTestItem("grandchild5", child3)
				grandchild6 := createTestItem("grandchild6", child3)

				greatgrandchild1 := createTestItem("greatgrandchild1", grandchild1)
				createTestItem("greatgrandchild2", grandchild1)
				greatgrandchild3 := createTestItem("greatgrandchild3", grandchild2)
				createTestItem("greatgrandchild4", grandchild3)
				createTestItem("greatgrandchild5", grandchild4)
				createTestItem("greatgrandchild6", grandchild4)
				greatgrandchild7 := createTestItem("greatgrandchild7", grandchild6)
				createTestItem("greatgrandchild8", grandchild6)

				createTestItem("greatgreatgrandchild1", greatgrandchild1)
				createTestItem("greatgreatgrandchild2", greatgrandchild1)
				createTestItem("greatgreatgrandchild3", greatgrandchild3)
				createTestItem("greatgreatgrandchild4", greatgrandchild7)

				return root
			}(),
			targetLevel:   4,
			expectedIDs:   []string{"greatgreatgrandchild1", "greatgreatgrandchild2", "greatgreatgrandchild3", "greatgreatgrandchild4"},
			expectedError: nil,
		},
		{
			name:          "Level not present",
			root:          createTestItem("root", nil),
			targetLevel:   1,
			expectedIDs:   nil,
			expectedError: nil,
		},
		{
			name: "Nil node",
			root: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
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
			item:        createTestItem("root", nil),
			expectedID:  "root",
			expectedErr: nil,
		},
		{
			name: "Child item with seed parent",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				return child
			}(),
			expectedID:  "root",
			expectedErr: nil,
		},
		{
			name: "Grandchild item with seed grandparent",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				grandchild := createTestItem("grandchild", child)
				return grandchild
			}(),
			expectedID:  "root",
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
			item:     createTestItem("root", nil),
			expected: 0,
		},
		{
			name: "Child item with depth 1",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				return child
			}(),
			expected: 1,
		},
		{
			name: "Grandchild item with depth 2",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				grandchild := createTestItem("grandchild", child)
				return grandchild
			}(),
			expected: 2,
		},
		{
			name: "Great-grandchild item with depth 3",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				grandchild := createTestItem("grandchild", child)
				greatGrandchild := createTestItem("greatGrandchild", grandchild)
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

func TestItem_GetDepthWithoutRedirections(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected int64
	}{
		{
			name:     "Seed item",
			item:     createTestItem("root", nil),
			expected: 0,
		},
		{
			name: "Top-level redirect item",
			item: func() *Item {
				root := createTestItem("root", nil)
				root.status = ItemGotRedirected
				child := createTestItem("child", root)
				child.status = ItemArchived
				return child
			}(),
			expected: 0,
		},
		{
			name: "Child item without redirections",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				return child
			}(),
			expected: 1,
		},
		{
			name: "Grandchild item without redirections",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				grandchild := createTestItem("grandchild", child)
				return grandchild
			}(),
			expected: 2,
		},
		{
			name: "Child item with redirection",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				child.status = ItemGotRedirected
				grandchild := createTestItem("grandchild", child)
				return grandchild
			}(),
			expected: 1,
		},
		{
			name: "Great-grandchild item with multiple redirections",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				child.status = ItemGotRedirected
				grandchild := createTestItem("grandchild", child)
				grandchild.status = ItemGotRedirected
				greatGrandchild := createTestItem("greatGrandchild", grandchild)
				return greatGrandchild
			}(),
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.GetDepthWithoutRedirections(); got != tt.expected {
				t.Errorf("GetDepthWithoutRedirections() = %v, want %v", got, tt.expected)
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
			item:     createTestItem("root", nil),
			expected: 0,
		},
		{
			name: "Seed with one child",
			item: func() *Item {
				root := createTestItem("root", nil)
				createTestItem("child", root)
				return root
			}(),
			expected: 1,
		},
		{
			name: "Seed with two levels of children",
			item: func() *Item {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				createTestItem("grandchild", child)
				return root
			}(),
			expected: 2,
		},
		{
			name: "Seed with multiple children at different levels",
			item: func() *Item {
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				child2 := createTestItem("child2", root)
				createTestItem("grandchild1", child1)
				createTestItem("grandchild2", child2)
				createTestItem("greatGrandchild", child1.children[0])
				return root
			}(),
			expected: 3,
		},
		{
			name:     "Seed with no children",
			item:     createTestItem("root", nil),
			expected: 0,
		},
		{
			name: "Seed with multiple children at same level",
			item: func() *Item {
				root := createTestItem("root", nil)
				createTestItem("child1", root)
				createTestItem("child2", root)
				createTestItem("child3", root)
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
				parent := createTestItem("parentID", nil)
				parent.status = ItemGotRedirected
				return parent
			}(),
			child:       createTestItem("childID", nil),
			from:        ItemGotRedirected,
			expectedErr: nil,
		},
		{
			name: "Valid AddChild with ItemGotChildren",
			parent: func() *Item {
				parent := createTestItem("parentID", nil)
				parent.status = ItemGotChildren
				return parent
			}(),
			child:       createTestItem("childID", nil),
			from:        ItemGotChildren,
			expectedErr: nil,
		},
		{
			name: "Invalid AddChild with wrong state",
			parent: func() *Item {
				parent := createTestItem("parentID", nil)
				parent.status = ItemFresh
				return parent
			}(),
			child:       createTestItem("childID", nil),
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
			item:        createTestItem("testID", nil),
			source:      ItemSourceInsert,
			expectedErr: nil,
		},
		{
			name:        "Set source for seed item to ItemSourceQueue",
			item:        createTestItem("testID", nil),
			source:      ItemSourceQueue,
			expectedErr: nil,
		},
		{
			name:        "Set source for seed item to ItemSourceHQ",
			item:        createTestItem("testID", nil),
			source:      ItemSourceHQ,
			expectedErr: nil,
		},
		{
			name:        "Set source for seed item to ItemSourcePostprocess",
			item:        createTestItem("testID", nil),
			source:      ItemSourcePostprocess,
			expectedErr: nil,
		},
		{
			name:        "Set source for seed item to ItemSourceFeedback",
			item:        createTestItem("testID", nil),
			source:      ItemSourceFeedback,
			expectedErr: nil,
		},
		{
			name:        "Set source for child item to ItemSourceInsert",
			item:        createTestItem("testID", createTestItem("parentID", nil)),
			source:      ItemSourceInsert,
			expectedErr: fmt.Errorf("source is invalid for a child"),
		},
		{
			name:        "Set source for child item to ItemSourceQueue",
			item:        createTestItem("testID", createTestItem("parentID", nil)),
			source:      ItemSourceQueue,
			expectedErr: fmt.Errorf("source is invalid for a child"),
		},
		{
			name:        "Set source for child item to ItemSourceHQ",
			item:        createTestItem("testID", createTestItem("parentID", nil)),
			source:      ItemSourceHQ,
			expectedErr: fmt.Errorf("source is invalid for a child"),
		},
		{
			name:        "Set source for child item to ItemSourcePostprocess",
			item:        createTestItem("testID", createTestItem("parentID", nil)),
			source:      ItemSourcePostprocess,
			expectedErr: nil,
		},
		{
			name:        "Set source for child item to ItemSourceFeedback",
			item:        createTestItem("testID", createTestItem("parentID", nil)),
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
			item:     createTestItem("testID", nil),
			status:   ItemFresh,
			expected: ItemFresh,
		},
		{
			name:     "Set status to ItemPreProcessed",
			item:     createTestItem("testID", nil),
			status:   ItemPreProcessed,
			expected: ItemPreProcessed,
		},
		{
			name:     "Set status to ItemArchived",
			item:     createTestItem("testID", nil),
			status:   ItemArchived,
			expected: ItemArchived,
		},
		{
			name:     "Set status to ItemFailed",
			item:     createTestItem("testID", nil),
			status:   ItemFailed,
			expected: ItemFailed,
		},
		{
			name:     "Set status to ItemCompleted",
			item:     createTestItem("testID", nil),
			status:   ItemCompleted,
			expected: ItemCompleted,
		},
		{
			name:     "Set status to ItemGotRedirected",
			item:     createTestItem("testID", nil),
			status:   ItemGotRedirected,
			expected: ItemGotRedirected,
		},
		{
			name:     "Set status to ItemGotChildren",
			item:     createTestItem("testID", nil),
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
			item:        createTestItem("testID", nil),
			err:         nil,
			expectedErr: nil,
		},
		{
			name:        "Set error to non-nil error",
			item:        createTestItem("testID", nil),
			err:         errors.New("test error"),
			expectedErr: errors.New("test error"),
		},
		{
			name:        "Set error to another non-nil error",
			item:        createTestItem("testID", nil),
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
		expected *Item
	}{
		{
			name: "Create seed item",
			id:   "testID",
			url:  &URL{Raw: "http://example.com"},
			via:  "seedViaTest",
			expected: &Item{
				id:      "testID",
				url:     &URL{Raw: "http://example.com"},
				seedVia: "seedViaTest",
				status:  ItemFresh,
			},
		},
		{
			name: "Create child item",
			id:   "childID",
			url:  &URL{Raw: "http://example.com/child"},
			via:  "",
			expected: &Item{
				id:      "childID",
				url:     &URL{Raw: "http://example.com/child"},
				seedVia: "",
				status:  ItemFresh,
			},
		},
		{
			name: "Create seed item with empty seedVia",
			id:   "testID2",
			url:  &URL{Raw: "http://example.com/2"},
			via:  "",
			expected: &Item{
				id:      "testID2",
				url:     &URL{Raw: "http://example.com/2"},
				seedVia: "",
				status:  ItemFresh,
			},
		},
		{
			name: "Create child item with non-empty seedVia",
			id:   "childID2",
			url:  &URL{Raw: "http://example.com/child2"},
			via:  "seedViaTest2",
			expected: &Item{
				id:      "childID2",
				url:     &URL{Raw: "http://example.com/child2"},
				seedVia: "seedViaTest2",
				status:  ItemFresh,
			},
		},
		{
			name:     "Create seed item with nil URL",
			id:       "testID3",
			url:      nil,
			via:      "",
			expected: nil,
		},
		{
			name:     "Create child item with empty ID",
			id:       "",
			url:      &URL{Raw: "http://example.com/child3"},
			via:      "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := NewItemWithID(tt.id, tt.url, tt.via)
			if tt.expected == nil && item != nil {
				t.Errorf("expected nil, got: %v", item)
			} else if item != nil {
				if item.id != tt.expected.id {
					t.Errorf("expected id: %v, got: %v", tt.expected.id, item.id)
				}
				if item.url == nil && tt.expected.url != nil {
					t.Errorf("expected url: %v, got: %v", tt.expected.url, item.url)
				}
				if item.url.Raw != tt.expected.url.Raw {
					t.Errorf("expected url: %v, got: %v", tt.expected.url.Raw, item.url.Raw)
				}
				if item.IsSeed() != tt.expected.IsSeed() {
					t.Errorf("expected seed: %v, got: %v", tt.expected.IsSeed(), item.IsSeed())
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
				parent := createTestItem("parentID", nil)
				parent.status = ItemGotRedirected
				child := createTestItem("childID", parent)
				return child
			}(),
			expected: true,
		},
		{
			name: "Item with parent having status ItemGotChildren",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				parent.status = ItemGotChildren
				child := createTestItem("childID", parent)
				return child
			}(),
			expected: false,
		},
		{
			name:     "Item with no parent",
			item:     createTestItem("testID", nil),
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

func TestItem_IsChild(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected bool
	}{
		{
			name: "Item with parent having status ItemGotChildren",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				parent.status = ItemGotChildren
				child := createTestItem("childID", parent)
				return child
			}(),
			expected: true,
		},
		{
			name: "Item with parent having status ItemGotRedirected",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				parent.status = ItemGotRedirected
				child := createTestItem("childID", parent)
				return child
			}(),
			expected: false,
		},
		{
			name:     "Item with no parent",
			item:     createTestItem("testID", nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.IsChild(); got != tt.expected {
				t.Errorf("IsChild() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConcurrentAddChild(t *testing.T) {
	parent := createTestItem("parentID", nil)
	parent.status = ItemGotChildren

	var wg sync.WaitGroup
	numChildren := 100
	wg.Add(numChildren)

	for i := range numChildren {
		go func(i int) {
			defer wg.Done()
			child := createTestItem(fmt.Sprintf("childID%d", i), nil)
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
func TestItem_HasRedirection(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected bool
	}{
		{
			name: "Item with one child having status ItemGotRedirected",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				child := createTestItem("childID", parent)
				parent.status = ItemGotRedirected
				parent.children = []*Item{child}
				return parent
			}(),
			expected: true,
		},
		{
			name: "Item with one child having status ItemGotChildren",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				child := createTestItem("childID", parent)
				parent.status = ItemGotChildren
				parent.children = []*Item{child}
				return parent
			}(),
			expected: false,
		},
		{
			name: "Item with no children",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				return parent
			}(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.HasRedirection(); got != tt.expected {
				t.Errorf("HasRedirection() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestItem_HasChildren(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		expected bool
	}{
		{
			name: "Item with one child having status ItemGotRedirected",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				child := createTestItem("childID", parent)
				parent.status = ItemGotRedirected
				parent.children = []*Item{child}
				return parent
			}(),
			expected: false,
		},
		{
			name: "Item with one child having status ItemGotChildren",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				child := createTestItem("childID", parent)
				parent.status = ItemGotChildren
				parent.children = []*Item{child}
				return parent
			}(),
			expected: true,
		},
		{
			name: "Item with no children",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				return parent
			}(),
			expected: false,
		},
		{
			name: "Item with multiple children",
			item: func() *Item {
				parent := createTestItem("parentID", nil)
				child1 := createTestItem("childID1", parent)
				child2 := createTestItem("childID2", parent)
				parent.status = ItemGotChildren
				parent.children = []*Item{child1, child2}
				return parent
			}(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.HasChildren(); got != tt.expected {
				t.Errorf("HasChildren() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// This test is to ensure that a combined usage of GetNodesAtLevel and GetMaxDepth is successful at returning the apex children
func TestItem_GetNodesAtLevel_GetMaxDepth(t *testing.T) {
	tests := []struct {
		name          string
		setupTree     func() *Item
		expectedDepth int64
		expectedNodes []string
	}{
		{
			name: "Simple tree",
			setupTree: func() *Item {
				root := createTestItem("root", nil)
				createTestItem("child1", root)
				createTestItem("child2", root)
				return root
			},
			expectedDepth: 1,
			expectedNodes: []string{"child1", "child2"},
		},
		{
			name: "Three level tree",
			setupTree: func() *Item {
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				createTestItem("child2", root)
				createTestItem("grandchild1", child1)
				return root
			},
			expectedDepth: 2,
			expectedNodes: []string{"grandchild1"},
		},
		{
			name: "Complex tree",
			setupTree: func() *Item {
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				child2 := createTestItem("child2", root)
				child3 := createTestItem("child3", root)
				grandchild1 := createTestItem("grandchild1", child1)
				createTestItem("grandchild2", child2)
				grandchild3 := createTestItem("grandchild3", child2)
				createTestItem("grandchild4", child3)
				createTestItem("greatgrandchild1", grandchild1)
				createTestItem("greatgrandchild2", grandchild3)
				return root
			},
			expectedDepth: 3,
			expectedNodes: []string{"greatgrandchild1", "greatgrandchild2"},
		},
		{
			name: "Single node tree",
			setupTree: func() *Item {
				return createTestItem("root", nil)
			},
			expectedDepth: 0,
			expectedNodes: []string{"root"},
		},
		{
			name: "Two level tree with multiple children",
			setupTree: func() *Item {
				root := createTestItem("root", nil)
				createTestItem("child1", root)
				createTestItem("child2", root)
				createTestItem("child3", root)
				return root
			},
			expectedDepth: 1,
			expectedNodes: []string{"child1", "child2", "child3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setupTree()

			// Get the apex children
			apexChildren, err := root.GetNodesAtLevel(root.GetMaxDepth())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check if the apex children are correct
			if len(apexChildren) != len(tt.expectedNodes) {
				t.Fatalf("expected %d apex children, got: %d", len(tt.expectedNodes), len(apexChildren))
			}

			expectedNodesMap := make(map[string]bool)
			for _, id := range tt.expectedNodes {
				expectedNodesMap[id] = true
			}

			for _, child := range apexChildren {
				if !expectedNodesMap[child.id] {
					t.Fatalf("unexpected apex child: %s", child.id)
				}
			}
		})
	}
}

func TestItem_RemoveChild(t *testing.T) {
	tests := []struct {
		name          string
		setupTree     func() (*Item, *Item)
		childToRemove *Item
		expectedIDs   []string
	}{
		{
			name: "Remove existing child",
			setupTree: func() (*Item, *Item) {
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				createTestItem("child2", root)
				return root, child1
			},
			expectedIDs: []string{"child2"},
		},
		{
			name: "Remove non-existing child",
			setupTree: func() (*Item, *Item) {
				root := createTestItem("root", nil)
				createTestItem("child1", root)
				createTestItem("child2", root)
				nonExistingChild := createTestItem("nonExistingChild", nil)
				return root, nonExistingChild
			},
			expectedIDs: []string{"child1", "child2"},
		},
		{
			name: "Remove child from single child",
			setupTree: func() (*Item, *Item) {
				root := createTestItem("root", nil)
				child := createTestItem("child", root)
				return root, child
			},
			expectedIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, childToRemove := tt.setupTree()
			root.RemoveChild(childToRemove)

			children := root.GetChildren()
			if len(children) != len(tt.expectedIDs) {
				t.Fatalf("expected %d children, got %d", len(tt.expectedIDs), len(children))
			}

			expectedIDsMap := make(map[string]bool)
			for _, id := range tt.expectedIDs {
				expectedIDsMap[id] = true
			}

			for _, child := range children {
				if !expectedIDsMap[child.id] {
					t.Fatalf("unexpected child: %s", child.id)
				}
			}
		})
	}
}

func TestItem_Traverse(t *testing.T) {
	tests := []struct {
		name       string
		setupTree  func() *Item
		traverseFn func(*Item)
		expected   []string
	}{
		{
			name: "Simple tree",
			setupTree: func() *Item {
				root := createTestItem("root", nil)
				createTestItem("child1", root)
				createTestItem("child2", root)
				return root
			},
			traverseFn: func(item *Item) {
				// This function will be used to collect the IDs of the traversed items
			},
			expected: []string{"root", "child1", "child2"},
		},
		{
			name: "Three level tree",
			setupTree: func() *Item {
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				createTestItem("child2", root)
				createTestItem("grandchild1", child1)
				return root
			},
			traverseFn: func(item *Item) {
				// This function will be used to collect the IDs of the traversed items
			},
			expected: []string{"root", "child1", "grandchild1", "child2"},
		},
		{
			name: "Complex tree",
			setupTree: func() *Item {
				root := createTestItem("root", nil)
				child1 := createTestItem("child1", root)
				child2 := createTestItem("child2", root)
				child3 := createTestItem("child3", root)
				grandchild1 := createTestItem("grandchild1", child1)
				createTestItem("grandchild2", child1)
				createTestItem("grandchild3", child2)
				createTestItem("grandchild4", child3)
				createTestItem("greatgrandchild1", grandchild1)
				return root
			},
			traverseFn: func(item *Item) {
				// This function will be used to collect the IDs of the traversed items
			},
			expected: []string{"root", "child1", "grandchild1", "greatgrandchild1", "grandchild2", "child2", "grandchild3", "child3", "grandchild4"},
		},
		{
			name: "Very large tree",
			setupTree: func() *Item {
				root := createTestItem("root", nil)
				for i := range 100 {
					child := createTestItem(fmt.Sprintf("child%d", i), root)
					for j := range 10 {
						createTestItem(fmt.Sprintf("grandchild%d", i*10+j), child)
					}
				}
				return root
			},
			traverseFn: func(item *Item) {
				// This function will be used to collect the IDs of the traversed items
			},
			expected: func() []string {
				ids := []string{"root"}
				for i := range 100 {
					ids = append(ids, fmt.Sprintf("child%d", i))
					for j := range 10 {
						ids = append(ids, fmt.Sprintf("grandchild%d", i*10+j))
					}
				}
				return ids
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setupTree()
			var traversedIDs []string
			traverseFn := func(item *Item) {
				traversedIDs = append(traversedIDs, item.id)
			}
			root.Traverse(traverseFn)

			if len(traversedIDs) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(traversedIDs))
			}

			for i, id := range tt.expected {
				if traversedIDs[i] != id {
					t.Fatalf("expected item %s at index %d, got %s", id, i, traversedIDs[i])
				}
			}
		})
	}
}

func TestCompleteAndCheck(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *Item
		expected bool
	}{
		{
			name: "Non-seed item",
			setup: func() *Item {
				return createTestItemWithStatus("item1", nil, ItemFresh)
			},
			expected: false,
		},
		{
			name: "Seed item already completed",
			setup: func() *Item {
				return createTestItemWithStatus("item1", nil, ItemCompleted)
			},
			expected: true,
		},
		{
			name: "Seed item with incomplete children",
			setup: func() *Item {
				root := createTestItemWithStatus("root", nil, ItemFresh)
				createTestItemWithStatus("child1", root, ItemFresh)
				createTestItemWithStatus("child2", root, ItemFresh)
				return root
			},
			expected: false,
		},
		{
			name: "Seed item with completed children",
			setup: func() *Item {
				root := createTestItemWithStatus("root", nil, ItemGotChildren)
				createTestItemWithStatus("child1", root, ItemCompleted)
				createTestItemWithStatus("child2", root, ItemCompleted)
				return root
			},
			expected: true,
		},
		{
			name: "Seed item with mixed status children",
			setup: func() *Item {
				root := createTestItemWithStatus("root", nil, ItemGotChildren)
				createTestItemWithStatus("child1", root, ItemCompleted)
				createTestItemWithStatus("child2", root, ItemFresh)
				return root
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := tt.setup()
			got := item.CompleteAndCheck()
			if got != tt.expected {
				t.Errorf("CompleteAndCheck() = %v, want %v", got, tt.expected)
			}
		})
	}
}
