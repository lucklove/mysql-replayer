package main

import (
	"context"
	"flag"
	"github.com/google/subcommands"
	"github.com/lucklove/mysql-replayer/bench"
	"github.com/lucklove/mysql-replayer/prepare"
	"os"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&prepare.PrepareCommand{}, "")
	subcommands.Register(&bench.BenchCommand{}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
