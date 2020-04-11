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
				hash := c.Args().First()
				if hash == "" {
					cli.ShowCommandHelpAndExit(c, "cat-file", 0)
				}
				cmdCatFile(hash)
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
		{
			Name: "commit",
			Action: func(c *cli.Context) error {
				message := c.Args().First()
				if message == "" {
					cli.ShowCommandHelpAndExit(c, "commit", 0)
				}
				cmdCommit(message)
				return nil
			},
		},
		{
			Name: "checkout",
			Action: func(c *cli.Context) error {
				dist := c.Args().First()
				if dist == "" {
					cli.ShowCommandHelpAndExit(c, "checkout", 0)
				}
				cmdCheckout(dist)
				return nil
			},
		},
		{
			Name: "log",
			Action: func(c *cli.Context) error {
				cmdLog()
				return nil
			},
		},
	}
	app.Run(os.Args)
}

const (
	HEAD_PATH = ".toygit/HEAD"
)

func cmdInit() {
	dir, _ := os.Getwd()

	if checkExist(dir + "/.toygit") {
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
	f.WriteString("ref: refs/heads/master")

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

func readObjectByHash(sha1Prefix string) []byte {
	if len(sha1Prefix) < 2 {
		fmt.Println("hash prefix must be 2 or more characters")
		return nil
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
		return nil
	}

	if len(objs) > 1 {
		fmt.Println("match too many objects")
		return nil
	}

	f, err := os.Open(objDir + "/" + objs[0].Name())
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer f.Close()
	buf := new(bytes.Buffer)
	if err := readZlib(buf, f); err != nil {
		fmt.Println(err)
		return nil
	}
	return buf.Bytes()
}

func catFile(sha1Prefix string) {
	data := readObjectByHash(sha1Prefix)
	if data == nil {
		return
	}
	strData := string(data)
	topIxd := strings.Index(strData, " ")
	if strData[:topIxd] == "blob" {
		nulIdx := strings.Index(strData, "\x00")
		fmt.Println(strData[nulIdx:])
	} else {
		fmt.Println(strData)
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
		fmt.Println(err)
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

type treeObject struct {
	files       map[string]*indexEntry
	directories map[string]*treeObject
}

func restoreTreeObject(sha string, tree *treeObject) {
	data := readObjectByHash(sha)
	sepData := strings.Split(string(data), "\n")

	for _, obj := range sepData {
		sepEntry := strings.Split(obj, " ")
		if len(sepEntry) == 1 {
			continue
		}
		sha := sepEntry[0]
		path := sepEntry[1]
		objType := sepEntry[2]
		if objType == "blob" {
			tree.files[path] = &indexEntry{sha: sha, path: path}
			continue
		}

		if obj, ok := tree.directories[path]; ok {
			restoreTreeObject(sha, obj)
		} else {
			tree.directories[path] = &treeObject{files: make(map[string]*indexEntry), directories: make(map[string]*treeObject)}
			restoreTreeObject(sha, tree.directories[path])
		}
	}
}

func readPrevTreeObject(sha string) treeObject {
	data := string(readObjectByHash(sha))
	sepData := strings.Split(data, "\n")
	treeSha := strings.Split(sepData[0], " ")[1]
	tree := &treeObject{files: make(map[string]*indexEntry), directories: make(map[string]*treeObject)}
	restoreTreeObject(string(treeSha), tree)
	return *tree
}

func writeTreeObject(tree treeObject) string {
	result := ""
	for fileName, file := range tree.files {
		result += file.sha + " " + fileName + " " + "blob" + "\n"
	}
	for dirName, dir := range tree.directories {
		result += writeTreeObject(*dir) + " " + dirName + " " + "tree" + "\n"
	}

	return hashObject([]byte(result), true)
}

func createTreeObject(resPath []string, tree *treeObject, entry indexEntry) {
	if len(resPath) == 1 {
		tree.files[resPath[0]] = &entry
		return
	}
	name := resPath[0]
	res := resPath[1:]
	if obj, ok := tree.directories[name]; ok {
		createTreeObject(res, obj, entry)
	} else {
		tree.directories[name] = &treeObject{files: make(map[string]*indexEntry), directories: make(map[string]*treeObject)}
		createTreeObject(res, tree.directories[name], entry)
	}
}

func readHead() (branchName string, sha string) {
	h, _ := ioutil.ReadFile(HEAD_PATH)
	head := bytes.Split(h, []byte(" "))
	if string(head[0]) != "ref:" {
		return "", string(h)
	}

	data := strings.Split(string(h), "/")
	br := data[len(data)-1]
	return string(br), readRef(br)
}

func writeHead(name string, isBranch bool) {
	var data string
	if isBranch {
		data = "ref: refs/heads/" + name
	} else {
		data = name
	}
	f, _ := os.Create(HEAD_PATH)
	defer f.Close()
	f.WriteString(data)
}

func readRef(name string) string {
	refPath := ".toygit/refs/heads/" + name
	if !checkExist(refPath) {
		f, _ := os.Create(refPath)
		defer f.Close()
		return ""
	}
	rf, _ := ioutil.ReadFile(refPath)
	return string(rf)
}

func writeRef(name string, sha string) {
	refPath := ".toygit/refs/heads/" + name
	f, _ := os.Create(refPath)
	defer f.Close()
	f.WriteString(sha)
}

func clearIndex() {
	idxPath := ".toygit/index"
	f, _ := os.Create(idxPath)
	f.Close()
}

func cmdCommit(message string) {
	br, prevCommitSha := readHead()
	var newTree treeObject

	if prevCommitSha == "" {
		newTree = treeObject{files: make(map[string]*indexEntry), directories: make(map[string]*treeObject)}
	} else {
		newTree = readPrevTreeObject(prevCommitSha)
	}

	entries := readIndexEntries()

	if len(entries) == 0 {
		fmt.Println("nothing staged")
		return
	}

	for _, entry := range entries {
		sepPath := strings.Split(entry.path, "/")
		createTreeObject(sepPath, &newTree, entry)
	}

	treeSha := writeTreeObject(newTree)

	result := ""
	result += "tree" + " " + treeSha + "\n"
	if prevCommitSha != "" {
		result += "parent" + " " + prevCommitSha + "\n"
	}
	result += "author" + " " + "author_name" + "\n"
	result += "committer" + " " + "commiter_name" + "\n\n"
	result += message

	commitSha := hashObject([]byte(result), true)
	writeRef(br, commitSha)
	clearIndex()
	fmt.Println("commit hash is " + commitSha)
}

func removeAllFileAndDir() {
	files, _ := ioutil.ReadDir("./")
	for _, file := range files {
		if file.Name() == ".toygit" {
			continue
		}
		os.RemoveAll(file.Name())
	}
}

func restoreWorkingTree(tree *treeObject, prevPath string) {
	for _, file := range tree.files {
		if !checkExist(prevPath) {
			os.MkdirAll(prevPath, 0777)
		}
		f, _ := os.Create(prevPath + "/" + file.path)
		data := readObjectByHash(file.sha)
		strData := string(data)
		nulIdx := strings.Index(strData, "\x00")
		f.WriteString(strData[nulIdx:])
	}

	for name, subTree := range tree.directories {
		restoreWorkingTree(subTree, prevPath+"/"+name)
	}
}

func checkExist(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func checkDist(dist string) (sha string, isBranch bool) {
	path := ".toygit/refs/heads/" + dist
	fmt.Println(path)
	if checkExist(path) {
		commitSha, _ := ioutil.ReadFile(path)
		return string(commitSha), true
	}
	return dist, false
}

func cmdCheckout(dist string) {
	sha, isBranch := checkDist(dist)
	removeAllFileAndDir()
	tree := readPrevTreeObject(sha)
	restoreWorkingTree(&tree, "./")
	clearIndex()
	if !isBranch {
		writeHead(sha, false)
	} else {
		writeHead(dist, true)
	}
}

func showCommitLog(sha string) {
	fmt.Println("commit " + sha)
	catFile(sha)
	prevCommit := string(readObjectByHash(sha))
	commit := strings.Split(prevCommit, "\n")[1]
	sepCommit := strings.Split(commit, " ")
	if sepCommit[0] == "parent" {
		showCommitLog(sepCommit[1])
	}
}

func cmdLog() {
	_, rootCommitSha := readHead()
	showCommitLog(rootCommitSha)
}
