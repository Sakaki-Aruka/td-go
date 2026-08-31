package extract

import (
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
)

// Key: AbsPath, Value: Hash
func GetHashFromFiles(dir string, recurse bool, ext []string, hasher hash.Hash) (map[string][]byte, error) {
	result := make(map[string][]byte)
	extset := make(map[string]struct{})
	for _, e := range ext {
		extset[e] = struct{}{}
	}

	var abs string
	if filepath.IsAbs(dir) {
		abs = dir
	} else {
		a, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		abs = a
	}

	if info, err := os.Stat(abs); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("Not directory (%s)", abs)
	}

	dirs := make([]string, 0)
	if recurse {
		CollectAllChildDirs(abs, &dirs)
	} else {
		dirs = append(dirs, abs)
	}
	if len(dirs) == 0 {
		return make(map[string][]byte), nil
	}

	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			return nil, err
		}

		for _, e := range entries {
			path := filepath.Join(d, e.Name())
			if _, ok := extset[filepath.Ext(e.Name())]; !ok {
				continue
			}
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			io.Copy(hasher, file)

			result[path] = hasher.Sum(nil)
			hasher.Reset()
		}
	}

	return result, nil
}
