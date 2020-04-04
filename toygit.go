package main

import (
	"fmt"
	"os"

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
	}
	app.Run(os.Args)
}

func cmdInit() {
	dir, _ := os.Getwd()
	fmt.Println("Initialized empty Toygit repository in " + dir)
}
