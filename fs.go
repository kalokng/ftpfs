// Package ftpfs implements http.FileSystem with a FTP connection.
//
// It can be used in http.FileServer.
package ftpfs

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/goftp/ftp"
)

// FS is a user logged in, FTP connection.
// It implements http.FileSystem.
//
// As it relays on FTP connection, it is not safe for concurrent use.
type FS ftp.ServerConn

// Open issues a LIST FTP command with name to FTP server.
func (fs *FS) Open(name string) (http.File, error) {
	sc := (*ftp.ServerConn)(fs)

	ls, err := sc.List(name)
	if err != nil {
		return nil, err
	}
	if len(ls) == 0 {
		// check if it really contains no files
		err := sc.ChangeDir(name)
		if err != nil {
			return nil, ErrNotFound
		}
	}

	if len(ls) == 1 && !isDir(ls[0]) && name == ls[0].Name {
		// it is a file
		return &ftpFile{
			sc:    sc,
			path:  name,
			size:  int64(ls[0].Size),
			entry: ftpEntry{ls[0]},
		}, nil
	}
	return newFtpDir(name, ls), nil
}

func isDir(e *ftp.Entry) bool {
	return e.Type == ftp.EntryTypeFolder
}

var (
	ErrNotFound = errors.New("File not found")    // Open will return this error when file not found
	ErrInvalid  = errors.New("invalid argument")  // Seek on ftpFile will return this error when offset < 0
	ErrReadDir  = errors.New("Read on directory") // Read / Seek on ftpDir will always return this error
	ErrReadFile = errors.New("Read on file")      // Readdir on ftpFile will always return this error
)

const bufLen = 1024

// ftpFile implements http.File
type ftpFile struct {
	sc    *ftp.ServerConn
	path  string
	size  int64
	entry ftpEntry

	offset     uint64
	next       uint64
	readCloser io.ReadCloser

	bufStart uint64
	buf      [bufLen]byte
}

func (f *ftpFile) Close() error {
	if f.readCloser == nil {
		return nil
	}
	err := f.readCloser.Close()
	if err == nil {
		f.readCloser = nil
	}
	return err
}

func (f *ftpFile) Read(b []byte) (n int, err error) {
	if f.next != f.offset {
		l := f.offset - f.bufStart
		if l > bufLen {
			l = bufLen
		}
		if f.next >= f.bufStart && f.next < f.bufStart+l {
			n = copy(b, f.buf[f.next-f.bufStart:l])
			f.next += uint64(n)
			return n, nil
		}
		if f.readCloser != nil {
			c := f.readCloser
			f.readCloser = nil
			// TODO: handle close connection correctly !?
			go c.Close()
		}
	}
	if f.readCloser == nil {
		f.readCloser, err = f.sc.RetrFrom(f.path, f.next)
		if err != nil {
			f.readCloser = nil
			return 0, err
		}
		f.offset = f.next
		f.bufStart = f.next
	}
	n, err = f.readCloser.Read(b)
	if f.offset-f.bufStart < bufLen {
		copy(f.buf[f.offset-f.bufStart:], b)
	}
	f.offset += uint64(n)
	f.next = f.offset
	return n, err
}

func (f *ftpFile) Seek(offset int64, whence int) (int64, error) {
	pos := offset
	switch whence {
	case os.SEEK_SET:
		//Nothing to do
	case os.SEEK_CUR:
		pos += int64(f.offset)
	case os.SEEK_END:
		pos += f.size
	}
	if pos < 0 {
		return int64(f.offset), ErrInvalid
	}
	if uint64(pos) == f.offset {
		// no change of position
		return int64(f.offset), nil
	}
	f.next = uint64(pos)
	return pos, nil
}

func (f *ftpFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, ErrReadFile
}

func (f *ftpFile) Stat() (os.FileInfo, error) {
	return f.entry, nil
}

// ftpDir implements http.File and os.FileInfo
type ftpDir struct {
	path string
	fi   []os.FileInfo
}

type ftpEntry struct{ *ftp.Entry }

func (e ftpEntry) Name() string       { return e.Entry.Name }
func (e ftpEntry) Size() int64        { return int64(e.Entry.Size) }
func (e ftpEntry) ModTime() time.Time { return e.Entry.Time }
func (e ftpEntry) IsDir() bool        { return isDir(e.Entry) }
func (e ftpEntry) Sys() interface{}   { return nil }

func (e ftpEntry) Mode() os.FileMode {
	var mode os.FileMode = 0644
	if e.IsDir() {
		mode |= os.ModeDir
	}
	return mode
}

func newFtpDir(path string, entries []*ftp.Entry) *ftpDir {
	b := make([]os.FileInfo, len(entries))
	for i, v := range entries {
		b[i] = ftpEntry{v}
	}
	return &ftpDir{path: path, fi: b}
}

func (d *ftpDir) Close() error {
	return nil
}

func (d *ftpDir) Read(b []byte) (n int, err error) {
	return 0, ErrReadDir
}

func (d *ftpDir) Seek(offset int64, whence int) (int64, error) {
	return 0, ErrReadDir
}

func (d *ftpDir) Readdir(count int) ([]os.FileInfo, error) {
	if count <= 0 || count > len(d.fi) {
		return d.fi, nil
	}
	return d.fi[:count], nil
}

func (d *ftpDir) Stat() (os.FileInfo, error) {
	return d, nil
}

func (d *ftpDir) Name() string       { return d.path }
func (d *ftpDir) Size() int64        { return 0 }
func (d *ftpDir) Mode() os.FileMode  { return os.ModeDir | 0644 }
func (d *ftpDir) ModTime() time.Time { return time.Time{} }
func (d *ftpDir) IsDir() bool        { return true }
func (d *ftpDir) Sys() interface{}   { return nil }
