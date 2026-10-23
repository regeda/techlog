package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/regeda/techlog/internal/seed"
	"github.com/regeda/techlog/internal/service"
)

func runSeed(ctx context.Context, args []string, out io.Writer) (err error) {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	sf := addStoreFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := sf.open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()
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
	_, err = fmt.Fprintf(out, "seeded %d new events (%d already present) in %s\n", inserted, skipped, sf.db)
	return err
}
