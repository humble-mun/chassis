//go:build !unix && !windows

package tls

import "runtime"

// statInfo has no implementation on platforms that expose neither a usable inode
// (unix) nor the Windows file metadata path (e.g. plan9, js/wasm, wasip1). These
// platforms are explicitly unsupported for TLS hot reloading, so we fail loudly
// at startup rather than silently degrading rotation detection. The error return
// is retained only to share the signature with the unix/windows variants.
func statInfo(string) (fileInfo, error) {
	panic("tls: hot reload is not supported on " + runtime.GOOS)
}
