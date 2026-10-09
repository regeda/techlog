// Command techlog runs the Techlog server: web UI, HTTP API and MCP on one port.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/regeda/techlog/internal/store"
	"github.com/regeda/techlog/internal/store/sqlite"
)

const usage = `Usage:
  techlog serve [--addr localhost:8080] [--db share/techlog.db] [--store sqlite]
  techlog seed  [--db share/techlog.db] [--store sqlite]

  serve  start the web UI (/), the HTTP API (/api/v1) and MCP (/mcp) on one port
  seed   load the Spring Spin example story (safe to run more than once)
`

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "techlog:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command\n\n%s", usage)
	}
	switch args[0] {
	case "serve":
		return runServe(ctx, args[1:], out)
	case "seed":
		return runSeed(ctx, args[1:], out)
	case "help", "-h", "--help":
		_, err := fmt.Fprint(out, usage)
		return err
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
}

// storeFlags are the flags shared by every command that opens the store.
type storeFlags struct{ db, kind string }

func addStoreFlags(fs *flag.FlagSet) *storeFlags {
	f := &storeFlags{}
	fs.StringVar(&f.db, "db", "share/techlog.db", "database path (sqlite)")
	fs.StringVar(&f.kind, "store", "sqlite", "storage backend: sqlite")
	return f
}

// open is the one place that knows the available backends. To add a backend,
// add a case here and a package under internal/store.
func (f *storeFlags) open() (store.Store, error) {
	switch f.kind {
	case "sqlite":
		s, err := sqlite.Open(f.db)
		if err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown store %q (available: sqlite)", f.kind)
	}
}
