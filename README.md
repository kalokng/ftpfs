# ftpfs
--
    import "github.com/kalokng/ftpfs"

Package ftpfs implements http.FileSystem with a FTP connection.

It can be used in http.FileServer.

## Usage

```go
var (
	ErrNotFound = errors.New("File not found")    // Open will return this error when file not found
	ErrInvalid  = errors.New("invalid argument")  // Seek on ftpFile will return this error when offset < 0
	ErrReadDir  = errors.New("Read on directory") // Read / Seek on ftpDir will always return this error
	ErrReadFile = errors.New("Read on file")      // Readdir on ftpFile will always return this error
)
```

#### type FS

```go
type FS ftp.ServerConn
```

FS is a user logged in, FTP connection. It implements http.FileSystem.

As it relays on FTP connection, it is not safe for concurrent use.

#### func (*FS) Open

```go
func (fs *FS) Open(name string) (http.File, error)
```
Open issues a LIST FTP command with name to FTP server.
