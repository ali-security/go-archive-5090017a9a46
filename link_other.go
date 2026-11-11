//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package archive

import "os"

// linkNoFollow creates newname as a hard link to oldname.
func linkNoFollow(oldname, newname string) error {
	return os.Link(oldname, newname)
}
