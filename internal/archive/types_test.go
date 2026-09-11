package archive

import (
	"encoding/json"
	"testing"
)

func TestNormalizePages(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		want          []string
		wantPageCount int
	}{
		{
			name:          "legacy flat pages array",
			content:       `{"pageCount":2,"pages":["aaa","bbb"]}`,
			want:          []string{"aaa", "bbb"},
			wantPageCount: 2,
		},
		{
			name: "schema 2 cPages",
			content: `{"pageCount":3,"cPages":{"pages":[
				{"id":"aaa"},{"id":"bbb"},{"id":"ccc"}]}}`,
			want:          []string{"aaa", "bbb", "ccc"},
			wantPageCount: 3,
		},
		{
			name: "deleted pages are skipped",
			content: `{"cPages":{"pages":[
				{"id":"aaa"},
				{"id":"bbb","deleted":{"timestamp":"1:2","value":1}},
				{"id":"ccc"}]}}`,
			want:          []string{"aaa", "ccc"},
			wantPageCount: 2,
		},
		{
			name: "flat array wins when both are present",
			content: `{"pages":["aaa"],"cPages":{"pages":[
				{"id":"bbb"},{"id":"ccc"}]}}`,
			want:          []string{"aaa"},
			wantPageCount: 0,
		},
		{
			name:          "no pages at all",
			content:       `{"pageCount":0}`,
			want:          nil,
			wantPageCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Content
			if err := json.Unmarshal([]byte(tt.content), &c); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			c.NormalizePages()

			if len(c.Pages) != len(tt.want) {
				t.Fatalf("got %d pages %v, want %d %v", len(c.Pages), c.Pages, len(tt.want), tt.want)
			}
			for i, page := range tt.want {
				if c.Pages[i] != page {
					t.Errorf("page %d: got %q, want %q", i, c.Pages[i], page)
				}
			}
			if c.PageCount != tt.wantPageCount {
				t.Errorf("pageCount: got %d, want %d", c.PageCount, tt.wantPageCount)
			}
		})
	}
}
