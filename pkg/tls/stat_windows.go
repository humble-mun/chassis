//go:build windows

package tls

import (
	"os"
)

// statInfo stats path and returns the rotation-detection snapshot.
// On Windows, inode is not available, so changed() falls back to size+modTime.
func statInfo(path string) (fileInfo, error) {
	st, err := os.Stat(path)
	if err != nil {
		return fileInfo{}, err
	}
	return fileInfo{size: st.Size(), modTime: st.ModTime().UnixNano()}, nil
}
