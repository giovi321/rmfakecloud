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

func TestPageTagsFormatVersion2(t *testing.T) {
	// formatVersion 2 writes objects in pageTags, not the bare strings the
	// earlier format used. Decoding the whole .content used to fail on this
	// field, leaving Content half populated.
	const content = `{"formatVersion":2,"pageTags":[
		{"name":"todo","pageId":"aaa","timestamp":1699999999},
		{"name":"idea","pageId":"bbb","timestamp":1700000000}],
		"cPages":{"pages":[{"id":"aaa"},{"id":"bbb"}]}}`

	var c Content
	if err := json.Unmarshal([]byte(content), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(c.Tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(c.Tags))
	}
	if c.Tags[0].Name != "todo" || c.Tags[0].PageID != "aaa" {
		t.Errorf("tag 0: got %+v", c.Tags[0])
	}

	c.NormalizePages()
	if len(c.Pages) != 2 {
		t.Errorf("got %d pages, want 2", len(c.Pages))
	}
}
