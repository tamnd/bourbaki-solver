package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/route"
)

const fleetUsage = `usage: bourbaki fleet <command> [flags]

commands:
  probe    ask every host what it has installed and how much of it is free
  accounts count the signed in profiles on each host and say how many can work
  ask      put one question to one host and print what came back
  up       start the ssh tunnels and wait for each to answer
  down     stop the tunnels
  status   print the tunnels, the health of each route and what it can carry
  bench    say what concurrency each host should carry, from the runs already done

flags:
  -routes PATH   route file (default $BOURBAKI_ROUTES, else ~/.config/bourbaki/routes.json)
  -only NAMES    comma separated route names, in place of every enabled route
  -state PATH    state file (default $BOURBAKI_FLEET_STATE, else ~/.config/bourbaki/fleet.json)

The taking column of accounts is the only one not read off the host. It is
answered out of recently asked, kept on this machine by the runs that did the
asking (default $BOURBAKI_FLEET_ASKS, else ~/.config/bourbaki/asks.json), and
it is there because every counted column can be right about a host that will
not take a prompt. One box read 21 verified with 10 ready and idle while 720 of
its asks in a week had ended "ChatGPT never accepted the prompt": the session
loads, the profile counts as ready, and the composer never takes it, so each
ask burns the whole deadline. Three extra translate lanes put against those ten
ready profiles wrote nothing at all. Where the record says that, the last
column says it instead of saying the host is free now, and the answer is to
sign the profiles in again rather than to point more lanes at them.

A dash in that column is a host nothing has asked lately, which is not the same
as a host that failed everything, and is what it reads before any run has gone.

Every chatgpt-tool listener binds 127.0.0.1 on its own host, so nothing here
works without ssh. The tunnels are what the rest of the tool talks to.

The model column of status is what the route asks for, checked against the
catalogue the host advertises. Both of those say what is on offer, and an
account that has been moved down still offers the same list, so status also
prints what the hosts have really answered on, read back off the finished jobs
in the queue. It costs no model calls and it knows nothing until work has run.
Read that block before starting anything long: a whole section of chapter VIII
came back on a cut down model with the board reading gpt-5 the whole time.
`

// fleetFlags are the three that every fleet subcommand takes.
type fleetFlags struct {
	routes string
	only   string
	state  string
}

func (f *fleetFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&f.routes, "routes", "", "route file")
	fs.StringVar(&f.only, "only", "", "comma separated route names")
	fs.StringVar(&f.state, "state", "", "state file")
}

func (f *fleetFlags) registry() (route.Registry, string, error) {
	registry, source, err := route.LoadOrDefault(f.routes)
	if err != nil {
		return registry, source, err
	}
	if strings.TrimSpace(f.only) != "" {
		registry, err = registry.Select(strings.Split(f.only, ","))
		if err != nil {
			return registry, source, err
		}
	}
	return registry, source, nil
}

// noSSHHost says that nothing is left to work with, and blames the right thing
// for it.
//
// The message used to name the route file whether or not the route file was the
// reason. -only narrows the registry before this is reached, so asking probe for
// a gateway or subscription route by name emptied the selection and was told
// "no route in routes.json names an ssh host" about a file holding four of them.
// That was read as the file being wrong and sent somebody to look at it; the
// answer was that the route asked for is not reachable over ssh and never was.
// A run against that same route then failed on the first ask, an hour later,
// with the model's own 400.
//
// The file is still to blame when nothing was selected, and then it is still
// named.
func (f *fleetFlags) noSSHHost(source, adjective string) error {
	if names := strings.TrimSpace(f.only); names != "" {
		return fmt.Errorf("no %sroute named in -only %s reaches a box over ssh, "+
			"so there is nothing to do here: gateway and subscription routes are asked directly",
			adjective, names)
	}
	return fmt.Errorf("no %sroute in %s names an ssh host", adjective, source)
}

func (f *fleetFlags) statePath() string {
	if strings.TrimSpace(f.state) != "" {
		return f.state
	}
	return fleet.StatePath()
}

func runFleet(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, fleetUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "probe":
		return runFleetProbe(args[1:])
	case "accounts":
		return runFleetAccounts(args[1:])
	case "ask":
		return runFleetAsk(args[1:])
	case "up":
		return runFleetUp(args[1:])
	case "down":
		return runFleetDown(args[1:])
	case "status":
		return runFleetStatus(args[1:])
	case "bench":
		return runFleetBench(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, fleetUsage)
		return nil
	default:
		return fmt.Errorf("unknown fleet command %q", args[0])
	}
}

// signalContext cancels on the first Ctrl-C so that a watch tears its tunnels
// down instead of orphaning them, and dies outright on the second.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runFleetProbe(args []string) error {
	var flags fleetFlags
	fs := flag.NewFlagSet("fleet probe", flag.ExitOnError)
	flags.bind(fs)
	timeout := fs.Duration("timeout", 30*time.Second, "per host timeout")
	save := fs.Bool("save", true, "write the facts to the state file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	registry, source, err := flags.registry()
	if err != nil {
		return err
	}

	// The ssh destination is not always the route name, and a route with no
	// host is a plain HTTP endpoint that has nothing to probe over ssh.
	targets := make([]fleet.Target, 0, len(registry.Routes))
	for _, value := range registry.Routes {
		if value.Host == "" {
			continue
		}
		targets = append(targets, fleetTarget(value))
	}
	if len(targets) == 0 {
		return flags.noSSHHost(source, "")
	}

	ctx, cancel := signalContext()
	defer cancel()
	rows := fleet.ProbeAll(ctx, fleet.SSH{Timeout: *timeout}, targets)
	fmt.Print(fleet.Table(rows))

	if !*save {
		return nil
	}
	path := flags.statePath()
	state, err := fleet.LoadState(path)
	if err != nil {
		return err
	}
	// Keyed by route name, which is the name the route file gives the box and
	// the name every report and every -hosts list uses. It was keyed by ssh
	// host, which was the same string for every host in the pool until one
	// arrived whose ssh stanza is not called what the route is: gamingpc is
	// reached as gpc, the facts went in under gpc, ocr run asked for gamingpc
	// and was told no chatgpt-tool path, run bourbaki doctor. The doctor had
	// already run and had the path in its hand.
	for index, target := range targets {
		state.Hosts[target.Name] = rows[index]
	}
	state.Written = time.Now().UTC()
	if err := state.Save(path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	return nil
}

// sshTargets is every route that names a box to log into. A route with no host
// is a plain HTTP endpoint and there is nothing on the far side to ask.
func sshTargets(registry route.Registry) []fleet.Target {
	targets := make([]fleet.Target, 0, len(registry.Routes))
	for _, value := range registry.Enabled() {
		if value.Host == "" {
			continue
		}
		targets = append(targets, fleetTarget(value))
	}
	return targets
}

// fleetTarget is the one place a route becomes something to ask over ssh.
//
// It exists because four call sites built the same three fields by hand and a
// fifth fact then had to reach all four. That fact is the kind: a route that
// names a reader is a box serving its own weights, and every question the fleet
// asks about readiness has a different answer for it. See fleet.Kind. Missing
// one call site would leave a host that is a reader everywhere except in the
// report that says whether it can read.
func fleetTarget(value route.Route) fleet.Target {
	target := fleet.Target{Name: value.Name, Host: value.Host, Port: value.RemotePort}
	if strings.TrimSpace(value.Reader) != "" {
		target.Kind = fleet.Reader
		target.ReaderURL = value.ReaderURL
		// The served model is what the endpoint calls the weights and is what a
		// row about a reader should print, since it is the thing that changes
		// when somebody swaps the shortlist entry under it.
		target.ReaderModel = value.ServedModel
		if target.ReaderModel == "" {
			target.ReaderModel = value.Model
		}
	}
	return target
}

// runFleetAccounts prints the ban board, and with -sleep prints the seconds a
// caller should wait before asking again.
//
// The board is the thing to look at before starting anything long. A browser
// profile that has read enough pages is rate limited for eight hours, and with
// every profile on a host sitting out, an OCR run does not fail fast: it sends
// its batch, waits for a composer that never appears, and gives up nineteen
// minutes later having read nothing. The sweep did that twice in a row. One
// ssh round trip says the same thing and says when to come back.
//
// Nothing here prints an account. The parser keeps counts and a duration and
// drops the addresses the host prints beside them, so a log tail from this can
// go into an issue as it stands.
func runFleetAccounts(args []string) error {
	var flags fleetFlags
	fs := flag.NewFlagSet("fleet accounts", flag.ExitOnError)
	flags.bind(fs)
	timeout := fs.Duration("timeout", 60*time.Second, "per host timeout")
	sleep := fs.Bool("sleep", false, "print the seconds to wait for the first slot back, and nothing else")
	within := fs.Duration("within", fleet.LedgerWindow, "how far back to read the record of asks already made")
	if err := fs.Parse(args); err != nil {
		return err
	}
	registry, source, err := flags.registry()
	if err != nil {
		return err
	}
	targets := sshTargets(registry)
	if len(targets) == 0 {
		return flags.noSSHHost(source, "enabled ")
	}

	ctx, cancel := signalContext()
	defer cancel()
	boards := fleet.BoardAll(ctx, fleet.SSH{Timeout: *timeout}, targets)
	if *sleep {
		fmt.Println(int(fleet.Wait(boards).Seconds()))
		return nil
	}
	// What the hosts hold, and then what asking them has lately come to. The
	// record is read here and not inside Board because it is written on this
	// machine by the runs that did the asking, and costs no ssh round trip.
	// Losing it is not worth failing the table for: a missing or unreadable
	// file means the taking column says nothing, which is what it says before
	// anything has run anyway.
	if ledger, err := fleet.LoadLedger(fleet.LedgerPath()); err != nil {
		fmt.Fprintf(os.Stderr, "the record of asks already made could not be read, so the taking column is empty: %v\n", err)
	} else {
		boards = ledger.Apply(boards, *within)
	}
	fmt.Print(fleet.AccountsTable(boards))
	if wait := fleet.Wait(boards); wait > 0 {
		fmt.Fprintf(os.Stderr, "every signed in profile is sitting out a cooldown, the first is back in %s\n",
			wait.Round(time.Minute))
	}
	for _, board := range boards {
		if board.NotTaking() {
			fmt.Fprintf(os.Stderr, "%s has %d ready profiles and %d of its last %d asks never reached a composer, so more lanes at it will write nothing: it wants signing in again\n",
				board.Host, board.Ready, board.NoComposer, board.Asks)
		}
	}
	return nil
}

func links(registry route.Registry, deep bool) []fleet.Link {
	prober := route.Prober{Deep: deep, Timeout: 10 * time.Second}
	out := make([]fleet.Link, 0, len(registry.Routes))
	for _, value := range registry.Enabled() {
		if value.Host == "" {
			continue
		}
		out = append(out, fleet.Link{
			Route:      value.Name,
			Host:       value.Host,
			LocalPort:  value.LocalPort,
			RemotePort: value.RemotePort,
			Check: func(ctx context.Context) error {
				health := prober.Probe(ctx, value)
				if health.State != route.StateLive {
					return fmt.Errorf("%s: %s", health.State, health.Detail)
				}
				return nil
			},
		})
	}
	return out
}

func runFleetUp(args []string) error {
	var flags fleetFlags
	fs := flag.NewFlagSet("fleet up", flag.ExitOnError)
	flags.bind(fs)
	watch := fs.Bool("watch", false, "stay in the foreground and restart tunnels that fail")
	poll := fs.Duration("poll", fleet.PollInterval, "how often to check a supervised tunnel")
	if err := fs.Parse(args); err != nil {
		return err
	}
	registry, source, err := flags.registry()
	if err != nil {
		return err
	}
	links := links(registry, false)
	if len(links) == 0 {
		return flags.noSSHHost(source, "enabled ")
	}

	ctx, cancel := signalContext()
	defer cancel()
	supervisor := &fleet.Supervisor{Logf: func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}}

	tunnels, err := supervisor.Up(ctx, links)
	if err != nil {
		return err
	}
	path := flags.statePath()
	state, err := fleet.LoadState(path)
	if err != nil {
		return err
	}
	state.Tunnels = tunnels
	state.Written = time.Now().UTC()
	if err := state.Save(path); err != nil {
		return err
	}
	for _, tunnel := range tunnels {
		fmt.Printf("%-8s  127.0.0.1:%d -> %s:%d  pid %d\n",
			tunnel.Route, tunnel.LocalPort, tunnel.Host, tunnel.RemotePort, tunnel.PID)
	}
	if !*watch {
		fmt.Fprintf(os.Stderr, "wrote %s, run bourbaki fleet down to stop\n", path)
		return nil
	}

	fmt.Fprintln(os.Stderr, "watching, Ctrl-C to stop")
	supervisor.Watch(ctx, links, *poll)
	// Ctrl-C in a watch means stop, and leaving the tunnels up would leave a
	// state file naming pids nothing is minding.
	for _, err := range supervisor.Down(tunnels) {
		fmt.Fprintln(os.Stderr, "down:", err)
	}
	state.Tunnels = nil
	state.Written = time.Now().UTC()
	return state.Save(path)
}

func runFleetDown(args []string) error {
	var flags fleetFlags
	fs := flag.NewFlagSet("fleet down", flag.ExitOnError)
	flags.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := flags.statePath()
	state, err := fleet.LoadState(path)
	if err != nil {
		return err
	}
	if len(state.Tunnels) == 0 {
		fmt.Println("no tunnels in", path)
		return nil
	}
	supervisor := &fleet.Supervisor{}
	for _, err := range supervisor.Down(state.Tunnels) {
		fmt.Fprintln(os.Stderr, "down:", err)
	}
	for _, tunnel := range state.Tunnels {
		fmt.Printf("%-8s  stopped pid %d\n", tunnel.Route, tunnel.PID)
	}
	state.Tunnels = nil
	state.Written = time.Now().UTC()
	return state.Save(path)
}

func runFleetStatus(args []string) error {
	var flags fleetFlags
	fs := flag.NewFlagSet("fleet status", flag.ExitOnError)
	flags.bind(fs)
	deep := fs.Bool("deep", false, "ask each route for a completion as well, which costs a minute or more per host")
	queueRoot := fs.String("queue", defaultQueueRoot(), "queue directory, read for what the hosts have answered on")
	if err := fs.Parse(args); err != nil {
		return err
	}
	registry, source, err := flags.registry()
	if err != nil {
		return err
	}
	state, err := fleet.LoadState(flags.statePath())
	if err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()
	pool := route.NewPool(registry)
	pool.Prober = route.Prober{Deep: *deep}
	results := pool.ProbeAll(ctx)

	fmt.Printf("routes from %s\n\n", source)
	fmt.Print(route.Table(results))
	fmt.Println()

	fmt.Printf("%-8s  %-6s  %-22s  %s\n", "route", "tunnel", "forward", "since")
	for _, value := range registry.Routes {
		tunnel, ok := state.Find(value.Name)
		up := "no"
		since, forward := "", fmt.Sprintf("%d:127.0.0.1:%d", value.LocalPort, value.RemotePort)
		if ok {
			// A pid in the file is a claim, not a fact. It survives a reboot,
			// and after one it names either nothing or somebody else.
			if tunnel.PID == 0 {
				up = "extern"
			} else if fleet.Alive(tunnel.PID) {
				up = "yes"
			} else {
				up = "stale"
			}
			since = tunnel.Started.Local().Format(time.RFC3339)
		}
		fmt.Printf("%-8s  %-6s  %-22s  %s\n", value.Name, up, forward, since)
	}

	if len(state.Hosts) > 0 {
		fmt.Println()
		rows := make([]fleet.Facts, 0, len(state.Hosts))
		for _, value := range registry.Routes {
			if facts, ok := state.Hosts[value.Name]; ok {
				facts.Name = value.Name
				// The kind comes off the route rather than out of the state
				// file, because the route file is the thing that declares it and
				// a state file written before there were kinds would read a
				// reader as a browser and print the wrong sentence about it.
				target := fleetTarget(value)
				facts.Kind, facts.ReaderURL = target.Kind.Or(), target.ReaderURL
				if facts.ReaderModel == "" {
					facts.ReaderModel = target.ReaderModel
				}
				rows = append(rows, facts)
			}
		}
		fmt.Print(fleet.Table(rows))
		fmt.Fprintf(os.Stderr, "host facts as of %s, run bourbaki fleet probe to refresh\n",
			state.Written.Local().Format(time.RFC3339))
	}
	printAnswers(*queueRoot, results)
	return nil
}
