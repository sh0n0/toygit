package main

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

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
				cmdHashObject(path, false)
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

func cmdHashObject(path string, write bool) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}

	sha := hashObject(data, "blob", write)
	fmt.Println(sha)
}

func hashObject(data []byte, objType string, write bool) string {
	header := objType + " " + strconv.Itoa(len(data)) + "\x00"
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
