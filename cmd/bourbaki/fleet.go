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
  up       start the ssh tunnels and wait for each to answer
  down     stop the tunnels
  status   print the tunnels, the health of each route and what it can carry
  bench    say what concurrency each host should carry, from the runs already done

flags:
  -routes PATH   route file (default $BOURBAKI_ROUTES, else ~/.config/bourbaki/routes.json)
  -only NAMES    comma separated route names, in place of every enabled route
  -state PATH    state file (default $BOURBAKI_FLEET_STATE, else ~/.config/bourbaki/fleet.json)

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
		targets = append(targets, fleet.Target{Name: value.Name, Host: value.Host, Port: value.RemotePort})
	}
	if len(targets) == 0 {
		return fmt.Errorf("no route in %s names an ssh host", source)
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
	// Keyed by ssh host, because that is what every later command has in hand
	// when it needs the chatgpt-tool path.
	for index, target := range targets {
		state.Hosts[target.Host] = rows[index]
	}
	state.Written = time.Now().UTC()
	if err := state.Save(path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
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
		return fmt.Errorf("no enabled route in %s names an ssh host", source)
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
			if facts, ok := state.Hosts[value.Host]; ok {
				facts.Name = value.Name
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
