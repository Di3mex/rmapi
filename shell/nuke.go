package shell

import (
	"errors"
	"fmt"

	"github.com/abiosoft/ishell"
	"github.com/juruen/rmapi/api"
)

func nukeCmd(ctx *ShellCtxt) *ishell.Cmd {
	return &ishell.Cmd{
		Name:      "nuke",
		Help:      "deletes everything",
		Completer: createEntryCompleter(ctx),
		Func: func(c *ishell.Context) {
			fmt.Print("Are you sure, this will DELETE EVERYTHING! type [YES]:")
			var response string
			_, err := fmt.Scanln(&response)
			if err != nil {
				return
			}
			if response != "YES" {
				return
			}
			fmt.Println("Nuking")
			err = api.Nuke(ctx.api.Http)

			if err != nil {
				c.Err(errors.New(fmt.Sprint("failed to delete entry", err)))
			}
		},
	}
}
