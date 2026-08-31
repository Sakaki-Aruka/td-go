package cmd

import (
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"td-go/extract"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

type option struct {
	HashDuplicateCheck     bool
	UseUuidFileName        bool
	NoDownloadLog          bool
	DownloadDestinationDir string
	IncludeExtensions      []string
}

func DownloadCmd() *cobra.Command {
	opt := option{}
	command := &cobra.Command{
		Use:   "dl [HarFile] [flags...]",
		Short: "Download files from a har file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			har := args[0]
			if info, err := os.Stat(har); err != nil {
				fmt.Printf("Invalid har file (%s)\n", har)
				return
			} else if info.IsDir() {
				fmt.Printf("'HarFile' must be a regular file path (%s)\n", har)
				return
			}

			if len(opt.IncludeExtensions) == 0 {
				fmt.Println("The extension list must not be empty")
				return
			}

			// Normalize extensions to always carry a leading dot so that the
			// link filter and the on-disk hash scan agree on the format.
			for i, e := range opt.IncludeExtensions {
				opt.IncludeExtensions[i] = "." + strings.TrimPrefix(e, ".")
			}

			if opt.DownloadDestinationDir == "" {
				if current, err := os.Getwd(); err == nil {
					opt.DownloadDestinationDir = current
				}
			}

			if info, err := os.Stat(opt.DownloadDestinationDir); err != nil {
				fmt.Printf("Invalid destination directory (%s)\n", opt.DownloadDestinationDir)
				return
			} else if !info.IsDir() {
				fmt.Printf("The destination dir must be a directory (%s)\n", opt.DownloadDestinationDir)
				return
			}

			dl(har, opt)
		},
	}

	command.Flags().BoolVar(&opt.HashDuplicateCheck, "hash", true, "Skip images whose content already exists in the destination")
	command.Flags().BoolVarP(&opt.UseUuidFileName, "uuid-style", "u", false, "Converting a filename to the uuid-style")
	command.Flags().BoolVarP(&opt.NoDownloadLog, "silent", "s", false, "Log silent")
	command.Flags().StringVarP(&opt.DownloadDestinationDir, "directory", "d", "", "Download files destination directory")
	command.Flags().StringSliceVarP(&opt.IncludeExtensions, "includes", "i", []string{".png", ".jpeg", ".jpg"}, "Include image extensions list")

	return command
}

// map: Key=Hash, Value=AbsPathes list
func getHash(dir string, ext []string, hasher hash.Hash) ([][]byte, error) {
	hash, err := extract.GetHashFromFiles(dir, false, ext, hasher)
	if err != nil {
		return nil, err
	}
	return slices.Collect(maps.Values(hash)), nil
}

func dl(har string, opt option) {
	links, err := extract.GetLinks(har)
	if err != nil {
		fmt.Printf("Failed to get image links")
		return
	}

	links = slices.DeleteFunc(links, func(l string) bool {
		for _, e := range opt.IncludeExtensions {
			if strings.HasSuffix(l, e) {
				return false
			}
		}
		return true
	})

	if len(links) == 0 {
		fmt.Println("No filtered links")
		return
	}

	hasher := sha512.New()
	hashset := make(map[[64]byte]struct{})
	if opt.HashDuplicateCheck {
		hash, err := getHash(opt.DownloadDestinationDir, opt.IncludeExtensions, hasher)
		if err != nil {
			fmt.Println("Failed to get file hash")
			return
		}

		for _, h := range hash {
			hashset[[64]byte(h)] = struct{}{}
		}
	}

	for _, l := range links {
		img, ok := fetch(l, opt)
		if !ok {
			continue
		}

		if opt.HashDuplicateCheck {
			hasher.Write(img)
			hash := [64]byte(hasher.Sum(nil))
			hasher.Reset()
			if _, exists := hashset[hash]; exists {
				if !opt.NoDownloadLog {
					fmt.Printf("Content already exists (%s)\n", l)
				}
				continue
			}
			hashset[hash] = struct{}{}
		}

		// link: https://pbs.twimg.com/media/<ImgId>.<Ext>
		elements := strings.Split(l, "/")
		filename := elements[len(elements)-1]
		if opt.UseUuidFileName {
			ext := filepath.Ext(filename)
			if u, err := uuid.NewRandom(); err == nil {
				filename = u.String() + ext
			}
		}
		path := filepath.Join(opt.DownloadDestinationDir, filename)

		if err := os.WriteFile(path, img, 0644); err != nil {
			if !opt.NoDownloadLog {
				fmt.Printf("Failed to save a file (%s)\n", path)
			}
			continue
		}

		if !opt.NoDownloadLog {
			fmt.Printf("Success to download and save (%s)\n", path)
		}
	}
}

func fetch(l string, opt option) ([]byte, bool) {
	resp, err := http.Get(l)
	if err != nil {
		if !opt.NoDownloadLog {
			fmt.Printf("Failed to download (%s)\n", l)
		}
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if !opt.NoDownloadLog {
			fmt.Printf("Failed to download (%s, %d)\n", l, resp.StatusCode)
		}
		return nil, false
	}

	img, err := io.ReadAll(resp.Body)
	if err != nil {
		if !opt.NoDownloadLog {
			fmt.Printf("Failed to read data from a response (%s)\n", l)
		}
		return nil, false
	}
	return img, true
}
