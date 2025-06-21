package github

import "testing"

func TestShouldConsiderAsAsset(t *testing.T) {
	cases := []struct {
		url      string
		expected bool
	}{
		{"https://avatars.githubusercontent.com/u/12345", true},
		{"https://github.githubassets.com/some-asset", true},
		{"https://github.com/user-attachments/file", true},
		{"https://github.com/user-or-org/repo/assets/image", true},
		{"https://private-user-images.githubusercontent.com/image", true},

		{"https://example.com/image.png", false},
		{"https://notgithub.com/image.png", false},
	}

	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			result := ShouldConsiderAsAsset(c.url)
			if result != c.expected {
				t.Errorf("ShouldConsiderAsAsset(%q) = %v; want %v", c.url, result, c.expected)
			}
		})
	}
}
