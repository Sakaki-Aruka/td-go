package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jessevdk/go-flags"
)

type Options struct {
	LinkSourceFile       string `short:"f" description:"source .har file" required:"true"`
	NoDuplicate          bool   `short:"n" description:"no duplicate download links"`
	OverrideExistsFile   bool   `short:"o" description:"overrides exists file"`
	UseUuidStyleFileName bool   `short:"u" description:"converting a file-name to an uuid-style"`
	LogSilent            bool   `short:"s" description:"log silent (no downloading log)"`
	TargetDir            string `short:"d" default:"" description:"specify a destination dir"`
	IncludeExtensions    string `short:"i" default:"png,jpg,jpeg" description:"include image extensions list (separated with comma)"`
}

func (optPtr *Options) GetIncludeExtensions() []string {
	if len(optPtr.IncludeExtensions) == 0 {
		return make([]string, 0)
	}
	return strings.Split(optPtr.IncludeExtensions, ",")
}

func (optPtr *Options) GetTargetDir() (string, error) {
	if len(optPtr.TargetDir) == 0 {
		return os.Getwd()
	}
	return filepath.Abs(optPtr.TargetDir)
}

func (optPtr *Options) String() string {
	source := "Source file: " + optPtr.LinkSourceFile
	targetDir, err := optPtr.GetTargetDir()
	if err == nil {
		targetDir = "Download target directory: " + targetDir
	} else {
		targetDir = "Download target directory: (Error)"
	}
	noDuplicate := "No duplicate: " + strconv.FormatBool(optPtr.NoDuplicate)
	override := "Override if a file exists: " + strconv.FormatBool(optPtr.OverrideExistsFile)
	useUuid := "Use UUID style file name: " + strconv.FormatBool(optPtr.UseUuidStyleFileName)
	logSilent := "No output download log: " + strconv.FormatBool(optPtr.LogSilent)
	includeExtensions := "Allowed extensions: " + strings.Join(optPtr.GetIncludeExtensions(), ", ")
	return strings.Join(
		[]string{source, targetDir, noDuplicate, override, useUuid, logSilent, includeExtensions},
		"\n")
}

func main() {
	/*
	 * arguments
	 *
	 * NEED
	 * - "-f FILE_NAME": specify a target .har file.
	 *
	 * OPTIONAL
	 * - "-n": no duplicate download links
	 * - "-u": converting a file-name to an uuid-style file-name.
	 * - "-s": log silent. (no downloading log)
	 * - "-d DIRECTORY": specify a destination directory.
	 * - "-i FILE,EXTENSIONS": include image extensions list that is separated with ",".

	 *
	 * NOT_IMPLEMENTED
	 *
	 * You can write some simple arguments with concatenated single string.
	 * e.g.) -nu
	 *
	 * If you write an argument that needs optional string argument with some simple arguments, you can use that ONLY on the end of parameters.
	 * e.g.) -nuf x.har
	 *
	 * You cannot write consecutive some arguments are require optional string arguments.
	 * e.g.) NG: -nufd x.har ./download
	 * e.g.) OK: -nuf x.har -d ./download/
	 *
	 */

	var opts Options
	opts.NoDuplicate = true
	optsPtr := &opts

	_, err := flags.ParseArgs(optsPtr, os.Args)
	if err != nil {
		os.Exit(4)
	}

	fmt.Println("=== Config ===\n" + optsPtr.String() + "\n")

	links, err := getLinks(optsPtr.LinkSourceFile, optsPtr)
	if err != nil {
		fmt.Println("Error occurred while get links. ")
		fmt.Println(err.Error())
		return
	}

	dlError := download(links, optsPtr)
	if dlError != nil {
		fmt.Println("Error occurred while downloading")
		fmt.Println(dlError.Error())
		return
	}
}

func getLinks(sourceFilePath string, optPtr *Options) ([]string, error) {
	file, err := os.Open(sourceFilePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	re, err := regexp.Compile(".*(https://pbs\\.twimg\\.com/media/.{15})\\?format=(.+)&name=.*")
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)
	var urls []string

	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		parsedRaw := re.FindAllStringSubmatch(string(line), -1)
		if len(parsedRaw) != 1 || len(parsedRaw[0]) != 3 {
			continue
		}
		baseUrl := parsedRaw[0][1]
		extension := parsedRaw[0][2]

		if !slices.Contains(optPtr.GetIncludeExtensions(), extension) {
			continue
		}

		link := baseUrl + "." + extension
		if optPtr.NoDuplicate && slices.Contains(urls, link) {
			continue
		}
		urls = append(urls, link)
	}

	return urls, nil
}

func download(links []string, optPtr *Options) error {
	for _, link := range links {
		response, err := http.Get(link)
		if err != nil {
			if !optPtr.LogSilent {
				fmt.Println("failed to get response")
				fmt.Println(err.Error())
			}
			continue
		}

		defer response.Body.Close()

		path, err := getWritePath(link, optPtr)
		if err != nil {
			if !optPtr.LogSilent {
				fmt.Println("failed to get a file path")
				fmt.Println(err.Error())
			}
			continue
		}

		_, checkError := os.Stat(path)
		if !optPtr.OverrideExistsFile && checkError == nil {
			// disallowed override && file found
			return checkError
		}

		if optPtr.OverrideExistsFile && checkError == nil {
			removeErr := os.Remove(path)
			if removeErr != nil {
				if !optPtr.LogSilent {
					fmt.Println("failed to override")
				}
				continue
			}
		}

		var file *os.File = nil
		if checkError != nil || optPtr.OverrideExistsFile {
			// file not found
			file, err = os.Create(path)
			if err != nil {
				return err
			}
		}
		defer file.Close()

		_, writeErr := io.Copy(file, response.Body)
		if writeErr != nil {
			fmt.Println("failed to write a response to a file")
			fmt.Println(writeErr.Error())
			continue
		}

		if !optPtr.LogSilent {
			fmt.Println("file downloaded: " + path)
		}
	}

	return nil
}

func getWritePath(link string, optPtr *Options) (string, error) {
	slashSplit := strings.Split(link, "/")
	if len(slashSplit) <= 0 {
		return "", errors.New("invalid link found. (link: " + link + ")")
	}

	filename := slashSplit[len(slashSplit)-1]

	dotSplit := strings.Split(filename, ".")
	if len(dotSplit) <= 0 {
		return "", errors.New("failed to get file extension")
	}
	extension := dotSplit[len(dotSplit)-1]
	if optPtr.UseUuidStyleFileName {
		randomName, err := uuid.NewRandom()
		if err != nil {
			return "", err
		}
		filename = randomName.String() + "." + extension
	}

	dir, err := optPtr.GetTargetDir()
	if err != nil {
		return "", err
	}
	path, err := url.JoinPath(dir, filename)
	if err != nil {
		return "", err
	}

	return path, nil
}
