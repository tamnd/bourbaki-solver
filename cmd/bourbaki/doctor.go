package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/route"
)

const doctorUsage = `usage: bourbaki doctor [flags]

Probes every enabled route and exits 0 if at least one of them can take work.
That makes it usable as a cron guard or a systemd ExecStartPre: no route means
no OCR, and starting a long run against a dead fleet wastes hours.

The probe is GET /v1/health and GET /v1/models, not a completion. The measured
round trip for nine tokens on this fleet is 151 seconds, so probing three hosts
by asking the model would take eight minutes. Pass -deep when the question is
whether the whole path works, and expect it to be slow.

flags:
  -routes PATH   route file (default $BOURBAKI_ROUTES, else ~/.config/bourbaki/routes.json)
  -only NAMES    comma separated route names
  -deep          ask each route for a completion as well
  -timeout D     per route timeout (default 30s, 6m with -deep)
  -quiet         print nothing, just set the exit status
`

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, doctorUsage) }
	path := fs.String("routes", "", "route file")
	only := fs.String("only", "", "comma separated route names")
	deep := fs.Bool("deep", false, "ask each route for a completion as well")
	timeout := fs.Duration("timeout", 0, "per route timeout")
	quiet := fs.Bool("quiet", false, "print nothing, just set the exit status")
	if err := fs.Parse(args); err != nil {
		return err
	}

	registry, source, err := route.LoadOrDefault(*path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*only) != "" {
		if registry, err = registry.Select(strings.Split(*only, ",")); err != nil {
			return err
		}
	}
	if key := os.Getenv(route.KeyEnv); strings.TrimSpace(key) == "" && !*quiet {
		// Worth saying before the probes rather than after three 401s.
		fmt.Fprintf(os.Stderr, "%s is not set, so every route will be refused\n", route.KeyEnv)
	}

	ctx, cancel := signalContext()
	defer cancel()
	pool := route.NewPool(registry)
	pool.Prober = route.Prober{Deep: *deep, Timeout: *timeout}
	results := pool.ProbeAll(ctx)

	if !*quiet {
		fmt.Printf("routes from %s\n\n", source)
		fmt.Print(route.Table(results))
	}

	var live int
	for _, result := range results {
		if result.State == route.StateLive {
			live++
		}
	}
	if live > 0 {
		if !*quiet {
			fmt.Printf("\n%d of %d routes can take work\n", live, len(results))
		}
		return nil
	}

	// Exit 1 with the reason on stderr, so a cron mail has something in it.
	for _, result := range results {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", result.Route, result.State, result.Detail)
		if !result.ResetsAt.IsZero() {
			fmt.Fprintf(os.Stderr, "  returns at %s, in %s\n",
				result.ResetsAt.Local().Format(time.RFC3339),
				time.Until(result.ResetsAt).Round(time.Minute))
		}
		if result.Drift != "" {
			fmt.Fprintf(os.Stderr, "  %s\n", result.Drift)
		}
	}
	return fmt.Errorf("no route can take work")
}
