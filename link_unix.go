//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package archive

import (
	"os"

	"golang.org/x/sys/unix"
)

// linkNoFollow creates newname as a hard link to oldname. If oldname is a
// symbolic link, the hard link refers to the symbolic link itself and not
// its target, also on platforms where link(2) follows symbolic links, so
// that a hard link cannot be used to reach a file outside the extraction
// root through a symlink.
func linkNoFollow(oldname, newname string) error {
	if err := unix.Linkat(unix.AT_FDCWD, oldname, unix.AT_FDCWD, newname, 0); err != nil {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: err}
	}
	return nil
}
