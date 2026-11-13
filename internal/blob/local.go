package blob

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
)

type local struct{ root string }

func newLocal(c Config) (backend, error) {
	if c.Root == "" {
		return nil, errors.New("local storage root is required")
	}
	if err := os.MkdirAll(c.Root, 0700); err != nil {
		return nil, errors.New("cannot create local storage root")
	}
	root, err := os.OpenRoot(c.Root)
	if err != nil {
		return nil, errors.New("cannot open local storage root")
	}
	root.Close()
	return &local{root: c.Root}, nil
}
func (l *local) put(ctx context.Context, key string, r io.Reader, size int64, _ string) error {
	root, err := os.OpenRoot(l.root)
	if err != nil {
		return err
	}
	defer root.Close()
	dir := path.Dir(key)
	if err = root.MkdirAll(dir, 0700); err != nil {
		return err
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		return err
	}
	temp := path.Join(dir, ".folio-upload-"+hex.EncodeToString(random))
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	_, copyErr := io.Copy(f, &exactReader{r: r, remaining: size})
	if copyErr == nil {
		copyErr = ctx.Err()
	}
	if copyErr == nil {
		copyErr = f.Sync()
	}
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename(temp, key)
}
func (l *local) open(ctx context.Context, key string) (io.ReadCloser, error) {
	root, err := os.OpenRoot(l.root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(key)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("object is not a regular file")
	}
	return f, nil
}
func (l *local) delete(ctx context.Context, key string) error {
	root, err := os.OpenRoot(l.root)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("object is not a regular file")
	}
	return root.Remove(key)
}
