package main

import (
	"fmt"
	"os"
)

// strongFileETag derives a strong validator from what ServeContent already
// stats: file size and nanosecond mtime. Setting it before ServeContent
// enables If-None-Match and byte-exact If-Range validation, which
// Last-Modified alone cannot provide within its one-second granularity
// (audit D5).
func strongFileETag(stat os.FileInfo) string {
	return fmt.Sprintf("\"%x-%x\"", stat.Size(), stat.ModTime().UnixNano())
}
