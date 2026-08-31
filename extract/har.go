package extract

import (
	"bufio"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
)

var linkPattern, _ = regexp.Compile(`.*(https://pbs.twimg.com/media/.{15})\?format=(.+)&name=.*`)

func GetLinks(path string) ([]string, error) {
	set := make(map[string]struct{})

	var abs string
	if filepath.IsAbs(path) {
		abs = path
	} else {
		a, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		abs = a
	}

	file, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// HAR files are single-line-per-entry but can embed large base64 bodies,
	// so lines may exceed bufio.Scanner's default token limit. Read with a
	// growable reader instead.
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			raws := linkPattern.FindAllStringSubmatch(line, -1)
			if len(raws) == 1 && len(raws[0]) == 3 {
				set[raws[0][1]+"."+raws[0][2]] = struct{}{}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}

	return slices.Collect(maps.Keys(set)), nil
}
