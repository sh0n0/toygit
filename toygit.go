package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "toygit"
	app.Commands = []cli.Command{
		{
			Name: "init",
			Action: func(c *cli.Context) error {
				cmdInit()
				return nil
			},
		},
		{
			Name: "hash-object",
			Action: func(c *cli.Context) error {
				path := c.Args().First()
				if path == "" {
					cli.ShowCommandHelpAndExit(c, "hash-object", 0)
				}
				cmdHashObject(path)
				return nil
			},
		},
		{
			Name: "cat-file",
			Action: func(c *cli.Context) error {
				path := c.Args().First()
				if path == "" {
					cli.ShowCommandHelpAndExit(c, "cat-file", 0)
				}
				cmdCatFile(path)
				return nil
			},
		},
		{
			Name: "add",
			Action: func(c *cli.Context) error {
				path := c.Args().First()
				if path == "" {
					cli.ShowCommandHelpAndExit(c, "add", 0)
				}
				cmdAdd(path)
				return nil
			},
		},
	}
	app.Run(os.Args)
}

func cmdInit() {
	dir, _ := os.Getwd()

	if _, err := os.Stat(dir + "/.toygit"); err == nil {
		fmt.Println("dir exists")
		return
	}

	os.MkdirAll(dir+"/.toygit/objects", 0777)
	os.MkdirAll(dir+"/.toygit/refs/heads", 0777)

	f, err := os.Create(dir + "/.toygit/HEAD")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	f.WriteString("ref: refs/heads/master\n")

	fmt.Println("Initialized empty Toygit repository in " + dir)
}

func cmdHashObject(path string) {
	fInfo, err := os.Stat(path)
	if err != nil {
		fmt.Println(err)
		return
	}

	if fInfo.IsDir() {
		fmt.Println("Unable to hash directory")
		return
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	header := "blob" + " " + strconv.Itoa(len(data)) + "\x00"
	result := append([]byte(header), data...)

	sha := hashObject(result, false)
	fmt.Println(sha)
}

func hashObject(result []byte, write bool) string {
	h := sha1.New()
	h.Write(result)
	sha := hex.EncodeToString(h.Sum(nil))

	if write {
		path := ".toygit/objects/" + sha[:2]
		os.Mkdir(path, 0777)

		f, err := os.Create(path + "/" + sha[2:])
		if err != nil {
			fmt.Println(err)
			return sha
		}
		defer f.Close()

		writeZlib(f, result)
	}
	return sha
}

func writeZlib(dst io.Writer, data []byte) {
	zw := zlib.NewWriter(dst)
	zw.Write(data)
	zw.Close()
}

func catFile(sha1Prefix string) {
	if len(sha1Prefix) < 2 {
		fmt.Println("hash prefix must be 2 or more characters")
		return
	}

	objDir := ".toygit/objects/" + sha1Prefix[:2]
	res := sha1Prefix[2:]

	files, _ := ioutil.ReadDir(objDir)
	objs := []os.FileInfo{}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), res) {
			objs = append(objs, f)
		}
	}

	if len(objs) == 0 {
		fmt.Println("not found")
		return
	}

	if len(objs) > 1 {
		fmt.Println("match too many objects")
		return
	}

	f, err := os.Open(objDir + "/" + objs[0].Name())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	buf := new(bytes.Buffer)
	if err := readZlib(buf, f); err != nil {
		fmt.Println(err)
		return
	}
	topIxd := strings.Index(buf.String(), " ")
	if buf.String()[:topIxd] == "blob" {
		nulIdx := strings.Index(buf.String(), "\x00")
		fmt.Println(buf.String()[nulIdx:])
	} else {
		fmt.Println(buf.String())
	}
}

func cmdCatFile(sha1Prefix string) {
	catFile(sha1Prefix)
	return
}

func readZlib(dst *bytes.Buffer, src io.Reader) error {
	r, err := zlib.NewReader(src)
	if err != nil {
		return err
	}

	if _, err := dst.ReadFrom(r); err != nil {
		return err
	}

	r.Close()
	return nil
}

// Index file: <sha> <path><\n>

type indexEntry struct {
	sha  string
	path string
}

func readIndexEntries() []indexEntry {
	idxPath := ".toygit/index"
	idx, _ := ioutil.ReadFile(idxPath)
	if len(idx) == 0 {
		return []indexEntry{}
	}

	entries := bytes.Split(idx, []byte("\n"))
	entries = entries[:len(entries)-1]

	allEntries := []indexEntry{}
	for _, entry := range entries {
		sepEntry := bytes.Split(entry, []byte(" "))
		newEntry := indexEntry{string(sepEntry[0]), string(sepEntry[1])}
		allEntries = append(allEntries, newEntry)
	}
	return allEntries
}

// visit recursively all directories and files under selected directory and return all file paths
func readAllFilePaths(root string) []string {
	allFilePaths := []string{}
	filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			if info.Name() == ".toygit" {
				return filepath.SkipDir
			}
			if info.Name() == ".git" {
				return filepath.SkipDir
			}

			if !info.IsDir() {
				allFilePaths = append(allFilePaths, path)
			}
			return nil
		})
	return allFilePaths
}

func cmdAdd(path string) {
	fInfo, err := os.Stat(path)
	if err != nil {
		println(err)
		return
	}

	stagedEntries := readIndexEntries()
	nextEntries := []indexEntry{}
	allFilePaths := []string{}

	if !fInfo.IsDir() {
		allFilePaths = append(allFilePaths, path)
	} else {
		allFilePaths = append(allFilePaths, readAllFilePaths(path)...)
	}

	for _, entry := range stagedEntries {
		exist := false
		for _, newPath := range allFilePaths {
			if entry.path == newPath {
				exist = true
			}
		}
		if !exist {
			nextEntries = append(nextEntries, entry)
		}
	}

	for _, newPath := range allFilePaths {
		data, err := ioutil.ReadFile(newPath)
		if err != nil {
			fmt.Println(err)
			return
		}
		header := "blob" + " " + strconv.Itoa(len(data)) + "\x00"
		result := append([]byte(header), data...)
		sha := hashObject(result, true)
		newEntry := indexEntry{sha, newPath}
		nextEntries = append(nextEntries, newEntry)
	}

	addEntriesToIndex(nextEntries)
}

func addEntriesToIndex(entries []indexEntry) {
	idxPath := ".toygit/index"
	data := ""

	for _, entry := range entries {
		data = data + entry.sha + " " + entry.path + "\n"
	}

	f, err := os.Create(idxPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(data)
	if err != nil {
		fmt.Println(err)
		return
	}
}
