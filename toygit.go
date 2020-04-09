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

	sha := hashObject(path, false)
	fmt.Println(sha)
}

func hashObject(path string, write bool) string {
	dir, _ := os.Getwd()
	data, err := ioutil.ReadFile(dir + "/" + path)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	header := "blob" + " " + strconv.Itoa(len(data)) + "\x00"
	result := append([]byte(header), data...)

	h := sha1.New()
	h.Write(result)
	sha := hex.EncodeToString(h.Sum(nil))

	if write {
		dir, _ := os.Getwd()
		path := dir + "/.toygit/objects/" + sha[:2]
		err := os.Mkdir(path, 0777)
		if err != nil {
			fmt.Println(err)
			return sha
		}

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

func cmdCatFile(sha1Prefix string) {
	if len(sha1Prefix) < 2 {
		fmt.Println("hash prefix must be 2 or more characters")
		return
	}

	dir, _ := os.Getwd()
	objDir := dir + "/.toygit/objects/" + sha1Prefix[:2]
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
	nulIdx := strings.Index(buf.String(), "\x00")
	fmt.Println(buf.String()[nulIdx:])

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
