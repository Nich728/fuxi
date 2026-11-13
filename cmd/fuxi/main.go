// Command fuxi is the single Fuxi binary (SYSTEM_DESIGN §1).
//
//	fuxi api              HTTP JSON API
//	fuxi worker           background jobs
//	fuxi migrate up       apply pending migrations (release step)
//	fuxi migrate down     roll back the latest migration
//	fuxi migrate status   list migrations
//	fuxi import ...       load CC-CEDICT, the HSK list or Tatoeba (§5)
//	fuxi healthcheck      exit 0 if the local API is ready (Docker probe)
//	fuxi version          print the build version
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nich728/fuxi/internal/platform/config"
	"github.com/Nich728/fuxi/internal/platform/logging"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `usage: fuxi <command>

commands:
  api                 run the HTTP API
  worker              run the background job worker
  migrate up|down|status
  import cedict|hsk|tatoeba <files>   load open datasets
  healthcheck         exit 0 if the local API is ready (for Docker)
  version             print the version
`

// healthcheck exits non-zero unless url (default: the local /readyz) answers 200.
func healthcheck(url string) error {
	if url == "" {
		url = "http://127.0.0.1:8080/readyz"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fuxi:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("missing command")
	}
	switch args[0] {
	case "version":
		fmt.Println(version)
		return nil
	case "healthcheck":
		// Used by Docker: the distroless image has no shell or curl.
		return healthcheck(os.Getenv("FUXI_HEALTHCHECK_URL"))
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(os.Stdout, cfg.LogLevel).With("version", version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "api":
		return runAPI(ctx, cfg, log.With("mode", "api"))
	case "worker":
		return runWorker(ctx, cfg, log.With("mode", "worker"))
	case "import":
		return runImport(ctx, cfg, log.With("mode", "import"), args[1:])
	case "migrate":
		sub := "up"
		if len(args) > 1 {
			sub = args[1]
		}
		return runMigrate(ctx, cfg, sub)
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}
