package main

import (
	"os"
	"flag"
	"context"
	"os/signal"
	"github.com/google/subcommands"
	"github.com/lucklove/mysql-replayer/bench"
	"github.com/lucklove/mysql-replayer/prepare"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&prepare.PrepareCommand{}, "")
	subcommands.Register(&bench.BenchCommand{}, "")

	flag.Parse()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	go func() {
		<- ch
		cancel()
	}()

	os.Exit(int(subcommands.Execute(ctx)))
}
