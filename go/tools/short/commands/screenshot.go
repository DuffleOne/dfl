package commands

import (
	"context"
	clilib "dfl/lib/cli"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/urfave/cli/v2"
)

const timeout = 1 * time.Minute

var Screenshot = &cli.Command{
	Name:  "screenshot",
	Usage: "Take a screenshot & upload it",

	Action: func(c *cli.Context) error {
		ctx, cancel := context.WithTimeout(c.Context, timeout)
		defer cancel()

		tmpName := fmt.Sprintf("%s-*.png", ksuid.Generate("file").String())
		out, err := ioutil.TempFile("", tmpName)
		if err != nil {
			return err
		}
		defer out.Close()

		err = exec.CommandContext(ctx, "screencapture", "-i", out.Name()).Run()
		if err != nil {
			return err
		}
		defer os.Remove(out.Name())

		tmpFile, err := os.Stat(out.Name())
		if os.IsNotExist(err) {
			return nil
		}

		if tmpFile.Size() == 0 {
			clilib.Notify("Cancelled", "No image was captured.")
			return nil
		}

		return c.App.Run([]string{"short", "signed-upload", out.Name()})
	},
}
