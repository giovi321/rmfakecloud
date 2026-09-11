package fs

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/ddvk/rmfakecloud/internal/storage"
	"github.com/ddvk/rmfakecloud/internal/storage/exporter"
	"github.com/ddvk/rmfakecloud/internal/storage/models"
)

// fakeBlobs serves blobs from memory.
type fakeBlobs map[string][]byte

func (f fakeBlobs) GetRootIndex() (string, int64, error) { return "", 0, nil }

func (f fakeBlobs) GetReader(hash string) (io.ReadCloser, error) {
	blob, ok := f[hash]
	if !ok {
		return nil, fmt.Errorf("no blob %s", hash)
	}
	return io.NopCloser(bytes.NewReader(blob)), nil
}

func rmPage(version string) []byte {
	// The header is 43 bytes, padded with spaces.
	header := fmt.Sprintf("reMarkable .lines file, version=%s", version)
	for len(header) < 43 {
		header += " "
	}
	return []byte(header)
}

func TestBlobPages(t *testing.T) {
	const content = `{"formatVersion":2,"cPages":{"pages":[
		{"id":"first"},{"id":"second"},{"id":"third"},{"id":"fourth"}]}}`

	blobs := fakeBlobs{
		"hcontent": []byte(content),
		"hfirst":   rmPage("6"),
		"hsecond":  rmPage("5"),
		"hfourth":  rmPage("6"),
	}

	doc := &models.HashDoc{
		HashEntry: models.HashEntry{EntryName: "doc"},
		Files: []*models.HashEntry{
			{Hash: "hcontent", EntryName: "doc" + storage.ContentFileExt},
			// Deliberately not in page order: the index is sorted by name.
			{Hash: "hfourth", EntryName: "doc/fourth" + storage.RmFileExt},
			{Hash: "hfirst", EntryName: "doc/first" + storage.RmFileExt},
			{Hash: "hsecond", EntryName: "doc/second" + storage.RmFileExt},
		},
	}

	pages, err := blobPages(doc, blobs)
	if err != nil {
		t.Fatalf("blobPages: %v", err)
	}

	// "third" has no blob, so it has to stay in place as a blank page,
	// otherwise "fourth" is exported as page 3.
	want := []exporter.RmVersion{
		exporter.VersionV6,
		exporter.VersionV5,
		exporter.VersionUnknown,
		exporter.VersionV6,
	}

	if len(pages) != len(want) {
		t.Fatalf("got %d pages, want %d", len(pages), len(want))
	}

	for i, version := range want {
		if pages[i].Version != version {
			t.Errorf("page %d: got version %v, want %v", i+1, pages[i].Version, version)
		}
		hasData := pages[i].Data != nil
		wantData := version != exporter.VersionUnknown
		if hasData != wantData {
			t.Errorf("page %d: hasData %v, want %v", i+1, hasData, wantData)
		}
	}
}

func TestBlobPagesFallsBackToIndexOrder(t *testing.T) {
	// A content file with no page list at all. The index is sorted by entry
	// name, which runs opposite to page order.
	blobs := fakeBlobs{
		"hcontent": []byte(`{"formatVersion":2}`),
		"ha":       rmPage("6"),
		"hb":       rmPage("6"),
	}

	doc := &models.HashDoc{
		HashEntry: models.HashEntry{EntryName: "doc"},
		Files: []*models.HashEntry{
			{Hash: "hcontent", EntryName: "doc" + storage.ContentFileExt},
			{Hash: "ha", EntryName: "doc/a" + storage.RmFileExt},
			{Hash: "hb", EntryName: "doc/b" + storage.RmFileExt},
		},
	}

	pages, err := blobPages(doc, blobs)
	if err != nil {
		t.Fatalf("blobPages: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
}

func TestBlobPayloadIsNilForANotebook(t *testing.T) {
	doc := &models.HashDoc{
		HashEntry: models.HashEntry{EntryName: "doc"},
		Files: []*models.HashEntry{
			{Hash: "hcontent", EntryName: "doc" + storage.ContentFileExt},
			{Hash: "ha", EntryName: "doc/a" + storage.RmFileExt},
		},
	}

	payload, err := blobPayload(doc, fakeBlobs{})
	if err != nil {
		t.Fatalf("blobPayload: %v", err)
	}
	if payload != nil {
		t.Error("a notebook has no payload, got one")
	}
}
