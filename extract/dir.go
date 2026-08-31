package extract

import (
	"fmt"
	"os"
	"path/filepath"
)

func CollectAllChildDirs(root string, result *[]string) {
	var abs string
	if filepath.IsAbs(root) {
		abs = root
	} else {
		a, err := filepath.Abs(root)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		abs = a
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, e := range entries {
		path := filepath.Join(abs, e.Name())
		if info, err := os.Stat(path); err != nil {
			return
		} else if info.IsDir() {
			*result = append(*result, path)
			CollectAllChildDirs(path, result)
		}
	}
}