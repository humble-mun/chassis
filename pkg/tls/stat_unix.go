//go:build unix

package tls

import (
	"os"
	"syscall"
)

// statInfo stats path and returns the rotation-detection snapshot.
func statInfo(path string) (fileInfo, error) {
	st, err := os.Stat(path)
	if err != nil {
		return fileInfo{}, err
	}
	fi := fileInfo{size: st.Size(), modTime: st.ModTime().UnixNano()}
	// os.Stat already performed the stat(2) syscall and filled Sys(); reading
	// Ino here is free and needs no cgo. On unusual filesystems the assertion
	// filesystems) the assertion fails and inode stays 0, so changed() falls
	// back to size+modTime.
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		fi.inode = uint64(sys.Ino)
	}
	return fi, nil
}
