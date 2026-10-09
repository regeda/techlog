package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/regeda/techlog/internal/seed"
	"github.com/regeda/techlog/internal/service"
)

func runSeed(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	sf := addStoreFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := sf.open()
	if err != nil {
		return err
	}
	defer st.Close()
	svc, err := service.New(st)
	if err != nil {
		return err
	}
	docs, err := seed.SpringSpin()
	if err != nil {
		return err
	}
	inserted, skipped, err := svc.Import(ctx, docs)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "seeded %d new events (%d already present) in %s\n", inserted, skipped, sf.db)
	return nil
}
