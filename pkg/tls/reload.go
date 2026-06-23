// Package tls provides reusable TLS building blocks for servers that load their
// key material from disk. Its primary feature today is hot reloading: server
// certificates and client-CA trust bundles are reloaded transparently when the
// underlying files are rotated in place (e.g. by cert-manager), so renewals take
// effect on the next handshake without a process restart.
//
// The package is named tls and intentionally shadows crypto/tls for callers;
// import crypto/tls under an alias if you need both in the same file.
package tls

import (
	"sync"

	"github.com/go-logr/logr"
)

// fileInfo is the subset of file metadata used to detect rotations cheaply.
// inode is the primary signal: cert-manager / kubelet rotate files by atomically
// swapping a "..data" symlink, which always changes the resolved inode. size and
// modTime are kept as a portable fallback for filesystems or platforms that do
// not expose a usable inode (inode == 0).
type fileInfo struct {
	inode   uint64
	size    int64
	modTime int64 // UnixNano
}

// reloader is the shared machinery behind CertReloader and CAReloader. It caches
// a value of type T loaded from one or more files and refreshes it when any of
// those files change on disk. The TOCTOU double-check, RWMutex, and
// log-and-fall-back-on-error behavior live here so each concrete reloader only
// supplies a load function.
//
// reloader is safe for concurrent use; current() is invoked from the TLS stack
// on every handshake.
type reloader[T any] struct {
	logger logr.Logger
	name   string   // human-readable subject for log messages, e.g. "TLS certificate"
	paths  []string // files watched for rotation; must be non-empty
	// load reads paths from disk and returns a freshly built value. It is only
	// called while the write lock is held.
	load func() (*T, error)

	mu    sync.RWMutex
	value *T
	infos []fileInfo // stat snapshots aligned with paths, set on last good load
}

// newReloader constructs a reloader and performs an initial load so a bad path
// fails fast at construction time.
func newReloader[T any](logger logr.Logger, name string, paths []string, load func() (*T, error)) (*reloader[T], error) {
	r := &reloader[T]{logger: logger, name: name, paths: paths, load: load}
	if _, err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// statAll snapshots every watched path. The first stat error is returned.
func (r *reloader[T]) statAll() ([]fileInfo, error) {
	infos := make([]fileInfo, len(r.paths))
	for i, p := range r.paths {
		fi, err := statInfo(p)
		if err != nil {
			return nil, err
		}
		infos[i] = fi
	}
	return infos, nil
}

// sameInfos reports whether a and b are element-wise equal.
func sameInfos(a, b []fileInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// reload re-reads the watched files and refreshes the cache, returning the
// freshly loaded value. The on-disk stat is re-taken under the write lock and
// compared again, so concurrent handshakes that all observed a change collapse
// into a single disk read (TOCTOU double-check).
func (r *reloader[T]) reload() (*T, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	infos, err := r.statAll()
	if err != nil {
		return nil, err
	}
	// Another goroutine may have already reloaded this exact revision while we
	// waited for the lock; if nothing changed, return the cached value without
	// touching disk.
	if r.value != nil && sameInfos(infos, r.infos) {
		return r.value, nil
	}
	value, err := r.load()
	if err != nil {
		return nil, err
	}
	r.value = value
	r.infos = infos
	return value, nil
}

// changed reports whether any watched file's stat snapshot differs from the
// cached one, indicating a rotation. A stat failure (e.g. mid-rotation) reports
// false so the cached value keeps being served.
func (r *reloader[T]) changed() bool {
	r.mu.RLock()
	cached := r.infos
	r.mu.RUnlock()

	cur, err := r.statAll()
	if err != nil {
		return false
	}
	return !sameInfos(cur, cached)
}

// current returns the cached value, reloading first if the watched files
// changed. On reload failure it logs and falls back to the last good value so a
// transient bad write mid-rotation never takes the listener down.
func (r *reloader[T]) current() *T {
	if r.changed() {
		value, err := r.reload()
		if err == nil {
			return value
		}
		// Fall through to the cached value, but make the failure observable: a
		// silently failing reload would otherwise only surface once the cached
		// material is finally retired.
		r.logger.Error(err, "reload failed, serving previous value", "name", r.name, "paths", r.paths)
	}
	r.mu.RLock()
	value := r.value
	r.mu.RUnlock()
	return value
}
