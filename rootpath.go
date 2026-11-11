package archive

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// errPathEscapes is returned when a path that is resolved within the
// extraction root leaves the root through a relative symlink. The message
// matches the error returned by os.Root for the same condition.
var errPathEscapes = errors.New("path escapes from parent")

var errTooManyLinks = errors.New("too many links")

// maxLinksWalked is the maximum number of symlinks followed when resolving a
// path within the extraction root, to protect against cycles.
const maxLinksWalked = 255

// resolveArchivePath resolves the parent directories of name within root,
// following symlinks as if root were the filesystem root. The final path
// component is intentionally preserved because archive extraction may create
// or replace it.
//
// name must be a cleaned, local path relative to root, using platform-native
// path separators. The result is a path relative to root that is guaranteed
// to stay within root; see resolveInRoot for the resolution rules.
//
// The path is resolved before the operation that uses it, so a concurrent
// change to the filesystem (other than by the archive being extracted) could
// still affect the operation. Archive entries are applied sequentially, so
// links created by the archive itself are always resolved within root.
func resolveArchivePath(root, name string) (string, error) {
	parent, base := filepath.Split(name)
	if parent == "" {
		return name, nil
	}
	resolvedParent, err := resolveInRoot(root, parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, base), nil
}

// resolveInRoot resolves all symlinks in name, a path relative to root, and
// returns the resolved path relative to root ("." for root itself).
//
//   - Relative symlinks are followed within root. A relative symlink that
//     resolves above root is rejected with errPathEscapes, matching os.Root.
//   - Absolute symlink targets are resolved relative to root (chroot-like),
//     as absolute symlinks are common in container root filesystems. Once an
//     absolute symlink has been followed, ".." components are clamped at
//     root instead of being rejected.
//   - Components that do not exist are retained, so they can be created.
//
// Every intermediate component is resolved by inspecting the filesystem
// below root only, so the returned path never refers to a location outside
// root, regardless of the symlinks found in it.
func resolveInRoot(root, name string) (string, error) {
	var (
		resolved    []string
		pending     = splitPath(name)
		linksWalked int
		followedAbs bool
	)
	for len(pending) > 0 {
		comp := pending[0]
		pending = pending[1:]

		switch comp {
		case "", ".":
			continue
		case "..":
			if len(resolved) > 0 {
				resolved = resolved[:len(resolved)-1]
			} else if !followedAbs {
				return "", &os.PathError{Op: "resolve", Path: name, Err: errPathEscapes}
			}
			continue
		}

		current := filepath.Join(filepath.Join(resolved...), comp)
		fullPath := filepath.Join(root, current) // #nosec G305 -- current is built from resolved components and stays within root.
		fi, err := os.Lstat(fullPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Path does not yet exist; it can be created later.
				resolved = append(resolved, comp)
				continue
			}
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			resolved = append(resolved, comp)
			continue
		}

		linksWalked++
		if linksWalked > maxLinksWalked {
			return "", &os.PathError{Op: "resolve", Path: name, Err: errTooManyLinks}
		}
		target, err := os.Readlink(fullPath)
		if err != nil {
			return "", err
		}
		if vol := filepath.VolumeName(target); vol != "" || (target != "" && os.IsPathSeparator(target[0])) {
			// Resolve absolute symlinks relative to root.
			followedAbs = true
			resolved = resolved[:0]
			target = target[len(vol):]
		}
		pending = append(splitPath(target), pending...)
	}
	if len(resolved) == 0 {
		return ".", nil
	}
	return filepath.Join(resolved...), nil
}

// splitPath splits p into its components, using the platform's path
// separators.
func splitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool {
		return r < utf8.RuneSelf && os.IsPathSeparator(uint8(r))
	})
}

// isOutsideRoot reports whether the cleaned path rel, relative to the
// extraction root, refers to a location outside of the root.
func isOutsideRoot(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
