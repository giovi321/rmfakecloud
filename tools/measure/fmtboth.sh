mkdir -p /tmp/f
for f in internal/storage/fs/blobstore.go internal/storage/fs/documents.go; do
  tr -d '\r' < "$f" > "/tmp/f/$(basename $f)"
done
gofmt -d /tmp/f/blobstore.go /tmp/f/documents.go | head -60
