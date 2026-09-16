mkdir -p /tmp/f
for f in internal/archive/types.go internal/archive/types_test.go internal/archive/reader.go internal/storage/fs/documents.go internal/storage/fs/blobstore.go internal/storage/models/archive.go; do
  tr -d '\r' < "$f" > "/tmp/f/$(basename $f)"
done
echo FILES-NEEDING-GOFMT:
gofmt -l /tmp/f/
echo END
