package commands

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/urfave/cli/v2"
)

var CopyURL = &cli.Command{
	Name:      "copy",
	ArgsUsage: "[url]",
	Aliases:   []string{"c"},
	Usage:     "Copy from a URL",

	Action: func(c *cli.Context) error {
		url, err := handleURLInput(c.Args().Slice())
		if err != nil {
			return err
		}

		filePath, err := downloadFile(url)
		if err != nil {
			return err
		}
		defer os.Remove(*filePath)

		return c.App.Run([]string{"short", "signed-upload", *filePath})
	},
}

func downloadFile(urlStr string) (*string, error) {
	fileToCopyRes, err := http.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer fileToCopyRes.Body.Close()

	tmpName := ksuid.Generate("tmpfile").String()

	out, err := ioutil.TempFile("", tmpName)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	_, err = io.Copy(out, fileToCopyRes.Body)
	if err != nil {
		return nil, err
	}

	path := out.Name()

	return &path, nil
}

func handleURLInput(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	url, err := urlPrompt.Run()
	if err != nil {
		return "", err
	}

	return url, nil
}
