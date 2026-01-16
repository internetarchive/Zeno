package connutil

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/discarder/contentlength"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	warc "github.com/internetarchive/gowarc"
)

// copyBufPool is a pool of 4KB byte slices used by CopyWithTimeout to avoid allocating a new buffer for every copy operation.
var copyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 4096)
		return &buf
	},
}

// BodyWithConn is a wrapper around resp.Body that also holds a reference to the warc.CustomConnection
// This is necessary to reset the read deadline after each read operation
type BodyWithConn struct {
	io.ReadCloser
	Conn *warc.CustomConnection
}

// CloseConnWithError call conn.CloseWithError(err) if conn is not nil and err is not nil and not equal to errFromConn.
// it always returns original error.
func CloseConnWithError(logger *log.FieldedLogger, conn *warc.CustomConnection, err error) error {
	if conn != nil && err != nil && !errors.Is(err, ErrFromConn) { // Avoid closing the connection twice if the error is from the connection itself
		if closeErr := conn.CloseWithError(err); closeErr != nil {
			if logger != nil {
				logger.Error("Failed to close connection with error", "originalErr", err, "closeErr", closeErr)
			}
		}
	}
	return err
}

var ErrFromConn = errors.New("this error is from the connection")

// CopyWithTimeout copies data and resets the read deadline after each successful read.
// NOTE: read deadline is handled by warc.CustomConnection in the background automatically
func CopyWithTimeout(dst io.Writer, src io.Reader) error {
	bufPtr := copyBufPool.Get().(*[]byte)
	buf := *bufPtr
	defer copyBufPool.Put(bufPtr)

	copied, maxContentLengthMiB := int64(0), config.Get().MaxContentLengthMiB
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			copied += int64(n)
			if maxContentLengthMiB > 0 && copied > int64(maxContentLengthMiB)*1024*1024 {
				return fmt.Errorf("%w: copied %d bytes, max-content-length is %d MiB", errors.New(contentlength.ContentLengthExceeded), copied, maxContentLengthMiB)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return errors.Join(ErrFromConn, err)
		}
	}
	return nil
}

// CopyWithTimeoutN copies a limited number of bytes and applies the timeout
// NOTE: read deadline is handled by warc.CustomConnection in the background automatically
func CopyWithTimeoutN(dst io.Writer, src io.Reader, n int64) error {
	_, err := io.CopyN(dst, src, n)
	if err != nil && err != io.EOF {
		return errors.Join(ErrFromConn, err)
	}
	return nil
}
