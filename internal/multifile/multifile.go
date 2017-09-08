package multifile

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"

	"github.com/takeyourhatoff/bt/internal/bencode"
)

// File is the interface returned by the methods in this package. Only the Close method is not safe for concurent use.
type File interface {
	io.ReaderAt
	io.WriterAt
	Size() int64
	io.Closer
}

// Open opens the torrent in the named directory for seeding.
func Open(dir string, i bencode.Metainfo) (File, error) {
	return open(dir, i, os.O_RDONLY, 0)
}

// Create creates the directory structure in the named directory, and creates and initilises the files of the torrent with the correct size, ready for downloading.
func Create(dir string, i bencode.Metainfo) (File, error) {
	return open(dir, i, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
}

// Continue opens the torrent in the named directory for continuing an aborted download. All the files in the torrent should already be present and initilised with the correct size.
func Continue(dir string, i bencode.Metainfo) (File, error) {
	return open(dir, i, os.O_RDWR, 0)
}

func open(dir string, i bencode.Metainfo, flag int, perm os.FileMode) (File, error) {
	name := filepath.Join(dir, filepath.Base(i.Info.Name))
	if i.Info.Length > 0 {
		f, err := os.OpenFile(name, flag, perm)
		if err != nil {
			return nil, errors.Wrap(err, "opening file")
		}
		if flag&os.O_CREATE != 0 {
			err = f.Truncate(i.Info.Length)
			if err != nil {
				return nil, errors.Wrap(err, "truncating file")
			}
		}
		return &sizeFile{f, i.Info.Length}, nil
	}
	mf := new(multiFile)
	var offset int64
	for _, fi := range i.Info.Files {
		fullName := filepath.Join(fi.Path...)
		fullName = filepath.Clean(fullName)
		fullName = filepath.Join(name, fullName)
		if flag&os.O_CREATE != 0 {
			err := os.MkdirAll(filepath.Dir(fullName), 0775)
			if err != nil {
				return nil, errors.Wrap(err, "creating directory")
			}
		}
		f, err := os.OpenFile(fullName, flag, perm)
		if err != nil {
			return nil, errors.Wrap(err, "opening file")
		}
		if flag&os.O_CREATE != 0 {
			err = f.Truncate(fi.Length)
			if err != nil {
				return nil, errors.Wrap(err, "truncating file")
			}
		}
		offset += fi.Length
		mf.files = append(mf.files, file{f, offset})
	}
	return mf, nil
}

type sizeFile struct {
	*os.File
	length int64
}

func (f *sizeFile) Size() int64 {
	return f.length
}

type file struct {
	f     *os.File
	limit int64
}

type multiFile struct {
	files []file
	sync.Mutex
}

func (mf *multiFile) offset(off int64) (f file, offset int64, err error) {
	for i, f := range mf.files {
		if f.limit == 0 || f.limit <= off {
			continue
		}
		if off > f.limit {
			break
		}
		var offset int64
		if i > 0 {
			offset = mf.files[i-1].limit
		}
		return f, offset, nil
	}
	if off == mf.Size() {
		return file{}, 0, io.EOF
	}
	return file{}, 0, errors.Errorf("offset %d out of range: 0 <= off < %d", off, mf.Size())
}

func (mf *multiFile) ReadAt(p []byte, off int64) (n int, err error) {
	f, foff, err := mf.offset(off)
	if err != nil {
		return 0, err
	}
	limit := off + int64(len(p))
	if limit > f.limit {
		p = p[:f.limit-off]
	}
	return f.f.ReadAt(p, off-foff)
}

func (mf *multiFile) WriteAt(p []byte, off int64) (n int, err error) {
	for len(p) > 0 {
		f, foff, err := mf.offset(off)
		if err != nil {
			return n, err
		}
		limit := off + int64(len(p))
		if limit > f.limit {
			limit = f.limit
		}
		n0, err := f.f.WriteAt(p[:limit-off], off-foff)
		n += n0
		if err != nil {
			return n, errors.WithStack(err)
		}
		off += int64(n0)
		p = p[n0:]
	}
	return n, nil
}

func (mf *multiFile) Size() int64 {
	return mf.files[len(mf.files)-1].limit
}

func (mf *multiFile) Close() error {
	var err error
	for _, f := range mf.files {
		err0 := f.f.Close()
		if err == nil {
			err = errors.WithStack(err0)
		}
	}
	return err
}
