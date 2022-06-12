package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/juruen/rmapi/api"
	"github.com/juruen/rmapi/log"
	"github.com/juruen/rmapi/shell"
)

const AUTH_RETRIES = 3

func run_shell(ctx func() api.ApiCtx, args []string) {
	err := shell.RunShell(ctx, args)

	if err != nil {
		log.Error.Println("Error: ", err)

		os.Exit(1)
	}
}

func main() {
	// log.InitLog()
	ni := flag.Bool("ni", false, "not interactive")
	flag.Parse()
	rstArgs := flag.Args()

	creator := func() api.ApiCtx {
		var ctx api.ApiCtx
		var err error
		for i := 0; i < AUTH_RETRIES; i++ {
			isSync15 := false
			ctx, isSync15, err = api.CreateApiCtx(api.AuthHttpCtx(i > 0, *ni))

			if err != nil {
				log.Trace.Println(err)
			} else {
				if isSync15 {
					fmt.Fprintln(os.Stderr, `WARNING!!!
	Using the new 1.5 sync, this has not been fully tested yet!!!
	Make sure you have a backup, in case there is a bug that could cause data loss!`)
				}
				break
			}
		}

		if err != nil {
			log.Error.Fatal("failed to build documents tree, last error: ", err)
		}
		return ctx
	}

	run_shell(creator, rstArgs)
}
