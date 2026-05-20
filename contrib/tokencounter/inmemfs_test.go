package tokencounter_test

import (
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/quenbyako/ext/fs"
)

type memFile struct {
	name   string
	data   []byte
	offset int64
}

func (f *memFile) Read(p []byte) (int, error) {
	if f.offset >= int64(len(f.data)) {
		return 0, io.EOF
	}

	n := copy(p, f.data[f.offset:])
	f.offset += int64(n)

	return n, nil
}

func (f *memFile) Stat() (fs.FileInfo, error) {
	return memFileInfo{name: f.name, size: int64(len(f.data))}, nil
}

func (f *memFile) Close() error { return nil }

type memWFile struct {
	fsys *MemFS
	name string
	buf  *bytes.Buffer
}

func (f *memWFile) Write(p []byte) (int, error) {
	return f.buf.Write(p) //nolint:wrapcheck // makes no sense to wrap buf error
}

func (f *memWFile) Stat() (fs.FileInfo, error) {
	return memFileInfo{name: f.name, size: int64(f.buf.Len())}, nil
}

func (f *memWFile) Close() error {
	f.fsys.mu.Lock()
	defer f.fsys.mu.Unlock()

	f.fsys.files[f.name] = f.buf.Bytes()

	return nil
}

type memFileInfo struct {
	name string
	size int64
}

func (i memFileInfo) Name() string       { return i.name }
func (i memFileInfo) Size() int64        { return i.size }
func (i memFileInfo) Mode() fs.FileMode  { return 0 }
func (i memFileInfo) ModTime() time.Time { return time.Time{} }
func (i memFileInfo) IsDir() bool        { return false }
func (i memFileInfo) Sys() any           { return nil }

type MemFS struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func NewMemFS() *MemFS {
	return &MemFS{
		files: make(map[string][]byte),
	}
}

func (m *MemFS) Open(name string) (fs.File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}

	return &memFile{name: name, data: data}, nil
}

func (m *MemFS) OpenW(name string) (fs.WFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return &memWFile{fsys: m, name: name, buf: bytes.NewBuffer(nil)}, nil
}

func (m *MemFS) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.files[name]; !ok {
		return fs.ErrNotExist
	}

	delete(m.files, name)

	return nil
}
