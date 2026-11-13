package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Nich728/fuxi/internal/dictionary"
	"github.com/Nich728/fuxi/internal/platform/config"
	"github.com/Nich728/fuxi/internal/platform/db"
)

const importUsage = `usage:
  fuxi import cedict  <cedict_ts.u8[.gz]>
  fuxi import hsk     <syllabus2025_words.tsv>
  fuxi import tatoeba <cmn_sentences.tsv[.bz2]> <eng_sentences.tsv[.bz2]> <cmn-eng_links.tsv[.bz2]>

Run them in this order. Re-running an import of the same file does nothing.`

// runImport loads an open dataset (SYSTEM_DESIGN §5). It runs in the
// foreground for now; the import.dataset job (§11) arrives with the job queue.
func runImport(ctx context.Context, cfg config.Config, log *slog.Logger, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", importUsage)
	}
	want := map[string]int{"cedict": 1, "hsk": 1, "tatoeba": 3}
	n, ok := want[args[0]]
	if !ok || len(args)-1 != n {
		return fmt.Errorf("%s", importUsage)
	}

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	im := dictionary.NewImporter(pool, log)

	var res dictionary.Result
	switch args[0] {
	case "cedict":
		res, err = im.ImportCedict(ctx, args[1])
	case "hsk":
		res, err = im.ImportHSK(ctx, args[1])
	case "tatoeba":
		res, err = im.ImportTatoeba(ctx, args[1], args[2], args[3])
	}
	if err != nil {
		return err
	}
	switch {
	case res.Skipped:
		fmt.Printf("%s: already imported, nothing to do\n", args[0])
	default:
		fmt.Printf("%s: %d rows inserted or changed\n", args[0], res.Rows)
	}
	if len(res.Unmatched) > 0 {
		sample := res.Unmatched[:min(len(res.Unmatched), 20)]
		fmt.Printf("%d HSK words have no CC-CEDICT entry, e.g. %s\n", len(res.Unmatched), strings.Join(sample, " "))
	}
	return nil
}
