package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"td-go/extract"

	"github.com/spf13/cobra"
)

func DedupeCmd() *cobra.Command {
	var dst string
	var ext []string

	command := &cobra.Command{
		Use:   "dedupe [dirs...] [flag]",
		Short: "Deduplicate",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("No target dirs")
				return
			}

			dirs := make([]string, 0)
			for _, d := range args {
				if info, err := os.Stat(d); err != nil || !info.IsDir() {
					fmt.Printf("Invalid directory path (%s)\n", d)
					continue
				} else {
					dirs = append(dirs, d)
				}
			}

			if len(dirs) == 0 {
				fmt.Println("No target directories")
				return
			}

			if dst == "" {
				if c, err := os.Getwd(); err == nil {
					dst = c
				}
			}

			if info, err := os.Stat(dst); err != nil || !info.IsDir() {
				fmt.Printf("Invalid destination directory path (%s)\n", dst)
				return
			}

			// Normalize extensions to always carry a leading dot so they match
			// filepath.Ext regardless of how the user wrote them (png / .png).
			for i, e := range ext {
				ext[i] = "." + strings.TrimPrefix(e, ".")
			}

			dedupe(dirs, dst, ext)
		},
	}

	command.Flags().StringVarP(&dst, "output", "o", "", "Output dir")
	command.Flags().StringSliceVarP(&ext, "extension", "e", []string{".png", ".jpeg", ".jpg"}, "Include extensions")
	return command
}

func dedupe(target []string, dst string, ext []string) {
	dirs := make(map[string]struct{})
	for _, d := range target {
		set := make([]string, 0)
		abs, _ := filepath.Abs(d)
		dirs[abs] = struct{}{}
		extract.CollectAllChildDirs(d, &set)
		for _, e := range set {
			dirs[e] = struct{}{}
		}
	}

	if len(dirs) == 0 {
		fmt.Println("No target directories")
		return
	}

	hashset := make(map[[64]byte]struct{})
	checkExt := len(ext) > 0
	count := 0
	for d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			fmt.Printf("Failed to get entries (%s)\n", d)
			continue
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}

			if checkExt {
				ok := false
				for _, x := range ext {
					if filepath.Ext(e.Name()) == x {
						ok = true
						break
					}
				}
				if !ok {
					continue
				}
			}
			path := filepath.Join(d, e.Name())
			img, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			HASHER.Write(img)
			h := [64]byte(HASHER.Sum(nil))
			HASHER.Reset()
			if _, ok := hashset[h]; ok {
				continue
			}

			savePath := filepath.Join(dst, strconv.Itoa(count)+filepath.Ext(e.Name()))
			if err := os.WriteFile(savePath, img, 0644); err != nil {
				fmt.Printf("Failed to save (%s)\n", savePath)
				continue
			}
			fmt.Printf("Success to dedupe (%s)\n", savePath)
			hashset[h] = struct{}{}
			count++
		}
	}
}
