package models

import (
	"testing"
)

func TestDrawTree(t *testing.T) {
	drawCreateTestItem := func(id string, parent *Item) *Item {
		item := &Item{
			id:     id,
			parent: parent,
		}
		if parent != nil {
			parent.children = append(parent.children, item)
		}
		return item
	}
	tests := []struct {
		name     string
		root     *Item
		expected string
	}{
		{
			name:     "Single node",
			root:     drawCreateTestItem("root", nil),
			expected: "root\n",
		},
		{
			name: "Two level tree",
			root: func() *Item {
				root := drawCreateTestItem("root", nil)
				drawCreateTestItem("child1", root)
				drawCreateTestItem("child2", root)
				return root
			}(),
			expected: "root\n├── child1\n└── child2\n",
		},
		{
			name: "Three level tree",
			root: func() *Item {
				root := drawCreateTestItem("root", nil)
				child1 := drawCreateTestItem("child1", root)
				drawCreateTestItem("child2", root)
				drawCreateTestItem("grandchild1", child1)
				return root
			}(),
			expected: "root\n├── child1\n│   └── grandchild1\n└── child2\n",
		},
		{
			name: "Complex tree",
			root: func() *Item {
				root := drawCreateTestItem("root", nil)
				child1 := drawCreateTestItem("child1", root)
				drawCreateTestItem("child2", root)
				grandchild1 := drawCreateTestItem("grandchild1", child1)
				drawCreateTestItem("grandchild2", child1)
				drawCreateTestItem("greatgrandchild1", grandchild1)
				return root
			}(),
			expected: "root\n├── child1\n│   ├── grandchild1\n│   │   └── greatgrandchild1\n│   └── grandchild2\n└── child2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.root.DrawTree()
			if result != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, result)
			}
		})
	}
}
