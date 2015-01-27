package walkablefs

import (
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/tools/godoc/vfs"
)

// WalkableFileSystem provides the standard filepath.Walk interface
type WalkableFileSystem interface {
	vfs.FileSystem
	Walk(root string, walkFn filepath.WalkFunc) error
}

// New returns the passed vfs.FileSystem back
// wrapped with Walker functionality
func New(fs vfs.FileSystem) WalkableFileSystem {
	return &walker{fs}
}

// walker wraps a vfs.FileSystem to Walk iterable filesystems
type walker struct {
	vfs.FileSystem
}

func (w walker) Walk(root string, walkFn filepath.WalkFunc) (err error) {
	info, err := w.Lstat(root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	return walk(w, root, info, walkFn)
}

func readDirNames(fs vfs.FileSystem, dirname string) ([]string, error) {
	if fis, err := fs.ReadDir(dirname); err != nil {
		return nil, err
	} else {
		var names []string
		for _, fi := range fis {
			names = append(names, fi.Name())
		}
		sort.Strings(names)
		return names, nil
	}
}

func walk(fs vfs.FileSystem, path string, info os.FileInfo, walkFn filepath.WalkFunc) error {
	err := walkFn(path, info, nil)
	if err != nil {
		if info.IsDir() && err == filepath.SkipDir {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	fnames, err := readDirNames(fs, path)
	if err != nil {
		return walkFn(path, info, err)
	}

	for _, name := range fnames {
		fn := filepath.Join(path, name)
		fi, err := fs.Lstat(fn)
		if err != nil {
			if err := walkFn(fn, fi, err); err != nil && err != filepath.SkipDir {
				return nil
			}
		} else {
			if err := walk(fs, fn, fi, walkFn); err != nil {
				if !fi.IsDir() || err != filepath.SkipDir {
					return err
				}
			}
		}
	}
	return nil
}
