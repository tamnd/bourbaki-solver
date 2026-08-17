// Package route names the hosts a run can send model calls to, orders them, and
// keeps the run moving when one of them stops answering.
//
// A single base URL is fine until the account behind it hits its daily limit,
// which on a ChatGPT web session is a routine event rather than an exceptional
// one. Naming the hosts makes the fallback order something you can read and
// argue with instead of something implicit in a shell script.
package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/api"
)

// Route is one host the fleet can send work to.
type Route struct {
	Name string `json:"name"`
	// Wire is the request format. There is one, because there is one thing on
	// the other end. The field exists so a route file that names something else
	// fails at load with a sentence rather than at the first call with a
	// transport error.
	Wire string `json:"wire"`
	// BaseURL may stop at the server root or end at /v1.
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	// APIKeyEnv names the environment variable holding the key, so a route file
	// can be committed or shared without carrying the secret itself.
	APIKeyEnv string `json:"api_key_env,omitempty"`
	// APIKey is a literal key passed on the command line. It never round trips
	// through a route file, because a file meant to be shared would leak it the
	// first time somebody pasted one.
	APIKey string `json:"-"`

	// Host is the ssh destination, which is what OCR needs: OCR does not go
	// over HTTP at all, it runs chatgpt-tool ocr-batch on the box. Empty means
	// the route is reachable only as an HTTP endpoint.
	Host string `json:"host,omitempty"`
	// RemotePort and LocalPort describe the tunnel. fleet up forwards
	// LocalPort to RemotePort on Host, and BaseURL is expected to name
	// LocalPort on the loopback.
	RemotePort int `json:"remote_port,omitempty"`
	LocalPort  int `json:"local_port,omitempty"`

	// Rank orders the routes, lowest first. It is not a quality score, it is
	// the order to try things in.
	Rank int `json:"rank"`
	// Concurrency is how many calls this host will take at once. It is measured
	// by fleet bench, not guessed.
	Concurrency int      `json:"concurrency,omitempty"`
	Timeout     Duration `json:"timeout,omitempty"`
	Disabled    bool     `json:"disabled,omitempty"`
	// Note carries why a route is ranked or disabled where it is. A disabled
	// row with no explanation reads as an oversight.
	Note string `json:"note,omitempty"`

	// Gateway says this route is a plain OpenAI-compatible endpoint rather than
	// a chatgpt-tool fronting a pool of browser sessions.
	//
	// Everything else in the table is the fleet, and the fleet answers /health
	// with the size of its pool, which is how a route is judged live and how a
	// host with nothing to answer with is caught before it is sent a chapter. A
	// public gateway has neither: no /health to ask and no pool to count. Asked
	// anyway it returns 404, and the route reads as broken when it is working.
	//
	// It is also what says the route cannot read a page image. OCR runs
	// chatgpt-tool ocr-batch over ssh and a gateway has no box to run it on, so
	// the two are not interchangeable and nothing should quietly try.
	Gateway bool `json:"gateway,omitempty"`

	// Command says this route is neither a box nor an endpoint but a program on
	// the machine the run is on, and names it.
	//
	// The subscription that the CLI in question speaks to is paid for already
	// and needs no browser, no rented box and no key in the environment. It is
	// a third kind of route because the two kinds above are both limits a run
	// spends its wall clock waiting on: a box runs out of turns and a free
	// gateway answers 429 with fourteen hours on it.
	//
	// A route with a command has no base URL and no wire, since nothing is sent
	// over a socket, and like a gateway it reads no page images.
	Command string `json:"command,omitempty"`
}

// WireChat is the only wire: POST /v1/chat/completions, streaming.
const WireChat = "chat"

// Duration is a time.Duration that round trips through JSON as "20m" rather
// than as a nanosecond count nobody can read.
type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(raw []byte) error {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		value, err := time.ParseDuration(text)
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", text, err)
		}
		*d = Duration(value)
		return nil
	}
	// A bare number is read as seconds, which is what someone hand editing a
	// route file most likely meant.
	var seconds float64
	if err := json.Unmarshal(raw, &seconds); err != nil {
		return fmt.Errorf(`duration must be a string like "20m" or a number of seconds`)
	}
	*d = Duration(time.Duration(seconds * float64(time.Second)))
	return nil
}

// Validate reports what is missing rather than letting the route fail at its
// first call with something obscure.
func (r Route) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("route has no name")
	}
	if strings.TrimSpace(r.Model) == "" {
		return fmt.Errorf("route %s has no model", r.Name)
	}
	if wire := strings.TrimSpace(r.Wire); wire != "" && wire != WireChat {
		return fmt.Errorf("route %s has unknown wire %q, want %q", r.Name, wire, WireChat)
	}
	// A command is run rather than called, so there is no address to be missing
	// and asking for one would refuse a route that works.
	if strings.TrimSpace(r.BaseURL) == "" && strings.TrimSpace(r.Command) == "" {
		return fmt.Errorf("route %s has no base_url", r.Name)
	}
	if r.Concurrency < 0 {
		return fmt.Errorf("route %s has negative concurrency", r.Name)
	}
	return nil
}

// Lanes is how many calls this route takes at once. A route file that omits it
// gets one, which is slow and correct, rather than unlimited, which is neither.
func (r Route) Lanes() int {
	if r.Concurrency > 0 {
		return r.Concurrency
	}
	return 1
}

// Registry is an ordered set of routes.
type Registry struct {
	Routes []Route `json:"routes"`
}

// DefaultPath is where the personal route file lives.
func DefaultPath() string {
	if value := strings.TrimSpace(os.Getenv("BOURBAKI_ROUTES")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "bourbaki", "routes.json")
}

// Load reads a registry from disk.
func Load(path string) (Registry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, err
	}
	var value Registry
	if err := json.Unmarshal(raw, &value); err != nil {
		return Registry{}, fmt.Errorf("decode route file %s: %w", path, err)
	}
	if err := value.Validate(); err != nil {
		return Registry{}, fmt.Errorf("route file %s: %w", path, err)
	}
	value.sort()
	return value, nil
}

// LoadOrDefault reads the named file, falls back to the personal one, and
// finally to the built-in fleet. A file named explicitly and missing is an
// error, because ignoring it quietly would run the wrong hosts.
func LoadOrDefault(path string) (Registry, string, error) {
	if strings.TrimSpace(path) != "" {
		registry, err := Load(path)
		return registry, path, err
	}
	if personal := DefaultPath(); personal != "" {
		registry, err := Load(personal)
		if err == nil {
			return registry, personal, nil
		}
		if !os.IsNotExist(err) {
			return Registry{}, personal, err
		}
	}
	return Default(), "built-in", nil
}

// Write saves a registry for editing.
func (r Registry) Write(path string) error {
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func (r Registry) Validate() error {
	if len(r.Routes) == 0 {
		return fmt.Errorf("route file lists no routes")
	}
	seen := map[string]bool{}
	ports := map[int]string{}
	for _, value := range r.Routes {
		if err := value.Validate(); err != nil {
			return err
		}
		if seen[value.Name] {
			return fmt.Errorf("route %s is listed twice", value.Name)
		}
		seen[value.Name] = true
		// Two tunnels on one local port is not a typo you find later. The
		// second ssh fails, the first keeps serving, and every call meant for
		// the second host quietly lands on the first.
		if value.LocalPort > 0 {
			if other, ok := ports[value.LocalPort]; ok {
				return fmt.Errorf("routes %s and %s both want local port %d", other, value.Name, value.LocalPort)
			}
			ports[value.LocalPort] = value.Name
		}
	}
	return nil
}

func (r *Registry) sort() {
	slices.SortStableFunc(r.Routes, func(a, b Route) int { return a.Rank - b.Rank })
}

// Enabled returns the usable routes in rank order. It sorts rather than trusting
// the caller to have done it, because a registry built in code is easy to write
// out of order and the resulting failover would be silently wrong.
func (r Registry) Enabled() []Route {
	var out []Route
	for _, value := range r.Routes {
		if !value.Disabled {
			out = append(out, value)
		}
	}
	slices.SortStableFunc(out, func(a, b Route) int { return a.Rank - b.Rank })
	return out
}

// Find returns the route with the given name.
func (r Registry) Find(name string) (Route, bool) {
	for _, value := range r.Routes {
		if value.Name == name {
			return value, true
		}
	}
	return Route{}, false
}

// Select restricts the registry to the named routes, in the order given. The
// caller's order beats rank, because naming routes explicitly is a statement
// about what to try first, and a named route that the file disables is included,
// since naming it is the override.
func (r Registry) Select(names []string) (Registry, error) {
	var out Registry
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		value, ok := r.Find(name)
		if !ok {
			return Registry{}, fmt.Errorf("unknown route %q, have %s", name, strings.Join(r.Names(), ", "))
		}
		value.Disabled = false
		out.Routes = append(out.Routes, value)
	}
	if len(out.Routes) == 0 {
		return Registry{}, fmt.Errorf("no routes selected")
	}
	return out, nil
}

// Names lists every route name in file order.
func (r Registry) Names() []string {
	out := make([]string, 0, len(r.Routes))
	for _, value := range r.Routes {
		out = append(out, value.Name)
	}
	return out
}

// Default is the fleet as measured on 2026-08-10, best first.
//
// Every number here was read off the hosts rather than guessed, and the ranks
// follow from them. server3 answers with twelve verified profiles at declared
// concurrency 4 on 8 cores and 15 GB free, so it leads. server2 answers with
// ten verified profiles at declared concurrency 3 on 6 cores and 7 GB free, so
// it is second. server1 has one profile, 4 cores and under a gigabyte free,
// which is not enough to run a browser, so it takes the overflow and no OCR.
//
// server2 was off when this file was first written and is on now: it had the
// tool, the accounts and the memory all along, and nothing was running serve.
// Bringing it up is a third of the fleet that was being left on the floor.
func Default() Registry {
	registry := Registry{Routes: []Route{
		{
			Name: "server3", Wire: WireChat, Host: "server3",
			BaseURL: "http://127.0.0.1:18773/v1", Model: DefaultModel,
			APIKeyEnv: KeyEnv, RemotePort: 8077, LocalPort: 18773,
			Rank: 10, Concurrency: 4, Timeout: Duration(20 * time.Minute),
			Note: "12 verified profiles, 8 cores, 15 GB free, browser",
		},
		{
			Name: "server2", Wire: WireChat, Host: "server2",
			BaseURL: "http://127.0.0.1:18772/v1", Model: DefaultModel,
			APIKeyEnv: KeyEnv, RemotePort: 8077, LocalPort: 18772,
			Rank: 20, Concurrency: 3, Timeout: Duration(20 * time.Minute),
			Note: "10 verified profiles, 6 cores, 7 GB free, browser",
		},
		{
			Name: "server1", Wire: WireChat, Host: "server1",
			BaseURL: "http://127.0.0.1:18771/v1", Model: DefaultModel,
			APIKeyEnv: KeyEnv, RemotePort: 8077, LocalPort: 18771,
			Rank: 30, Concurrency: 1, Timeout: Duration(20 * time.Minute),
			Note: "1 profile, 4 cores, under a gigabyte free, no OCR",
		},
		{
			Name: "zen", Wire: WireChat, Gateway: true,
			BaseURL: ZenBaseURL, Model: ZenModel, APIKeyEnv: ZenKeyEnv,
			Rank: 40, Concurrency: 2, Timeout: Duration(5 * time.Minute),
			Note: "free text models, no vision, ranked last so nothing moves to it by accident",
		},
		{
			Name: "zen-hy3", Wire: WireChat, Gateway: true,
			BaseURL: ZenBaseURL, Model: ZenHy3, APIKeyEnv: ZenKeyEnv,
			Rank: 50, Concurrency: 2, Timeout: Duration(5 * time.Minute),
			Note: "the same gateway on a second free model, so a second free allowance",
		},
		{
			Name: "zen-laguna", Wire: WireChat, Gateway: true,
			BaseURL: ZenBaseURL, Model: ZenLaguna, APIKeyEnv: ZenKeyEnv,
			Rank: 60, Concurrency: 2, Timeout: Duration(5 * time.Minute),
			Note: "and a third",
		},
		{
			Name: "zen-deepseek", Wire: WireChat, Gateway: true,
			BaseURL: ZenBaseURL, Model: ZenDeepseek, APIKeyEnv: ZenKeyEnv,
			Rank: 70, Concurrency: 2, Timeout: Duration(5 * time.Minute),
			Note: "a fourth allowance on a cut down model, which L08 will say so about",
		},
		{
			Name: "zen-mimo", Wire: WireChat, Gateway: true,
			BaseURL: ZenBaseURL, Model: ZenMimo, APIKeyEnv: ZenKeyEnv,
			Rank: 80, Concurrency: 2, Timeout: Duration(5 * time.Minute),
			Note: "a fifth allowance",
		},
		{
			Name: "zen-lightning", Wire: WireChat, Gateway: true,
			BaseURL: ZenBaseURL, Model: ZenLightning, APIKeyEnv: ZenKeyEnv,
			Rank: 90, Concurrency: 2, Timeout: Duration(5 * time.Minute),
			Note: "the sixth and last, and the quickest, so it is ranked behind the rest",
		},
		{
			Name: "codex-mini", Command: CodexCommand, Model: CodexMini,
			Rank: 100, Concurrency: 2, Timeout: Duration(5 * time.Minute),
			Note: "the subscription on this machine, on the cheap model, which is tried first and which L08 reports",
		},
		{
			Name: "codex", Command: CodexCommand, Model: CodexFull,
			Rank: 110, Concurrency: 2, Timeout: Duration(5 * time.Minute),
			Note: "the same subscription on the full model, which answers what the cheap one gets wrong",
		},
	}}
	registry.sort()
	return registry
}

// DefaultModel is the strongest slug the pool exposes.
const DefaultModel = "gpt-5"

// KeyEnv is the environment variable every route reads its key from. The key
// itself is never written to a file this repo can see.
const KeyEnv = "BOURBAKI_PROXY_KEY"

// The zen gateway is an OpenAI-compatible endpoint with a handful of models
// that cost nothing to call. It is here because the fleet's scarce resource is
// uploads, not tokens: reading a page image spends one, and a host that runs
// out is out for hours. Translating a volume, solving an exercise and asking
// for the Vietnamese of a term are all text, and text sent here is text the
// fleet does not have to pay an upload for.
//
// It cannot read a page. None of the free models on this gateway accepts an
// image, which was measured and not assumed: every one of them answers a
// request carrying an image with "404 No endpoints found that support image
// input". So OCR stays on the fleet and everything downstream of it can move.
const (
	ZenBaseURL = "https://opencode.ai/zen/v1"
	// ZenModel is the strongest of the free slugs. The gateway lists sixty two
	// models and six of them are free, and the others are metered, which this
	// corpus does not spend.
	ZenModel = "nemotron-3-ultra-free"
	// The other free slugs. The allowance is per model and not per account,
	// which was measured rather than assumed: with nemotron answering 429
	// FreeUsageLimitError to every call, deepseek, hy3 and laguna all answered
	// 200 in the same minute. So a run that has exhausted one of them has five
	// more to go at, and a translation of a volume that would have sat on its
	// backoff for the rest of the day carries on. All six are here because they
	// do not come back together: probed twice an hour apart, the first time only
	// hy3 answered and the second time hy3 and laguna did, and the four routes
	// this had before left two allowances on the table both times. They are separate routes
	// rather than a list on one, because the pool ranks and counts routes and a
	// route that is out of turns has to be the thing that is out.
	//
	// ZenDeepseek is a cut down model by its name, so quality.SmallModel says
	// so, L08 reports any section it writes, and it is ranked last of the four.
	ZenHy3       = "hy3-free"
	ZenLaguna    = "laguna-s-2.1-free"
	ZenDeepseek  = "deepseek-v4-flash-free"
	ZenMimo      = "mimo-v2.5-free"
	ZenLightning = "nemotron-3.5-lightning-free"
	// ZenKeyEnv is the variable opencode itself writes, so a machine that has
	// the CLI set up has the route working already.
	ZenKeyEnv = "OPENCODE_API_KEY"
)

// The subscription this machine is signed in to, reached by running the codex
// CLI rather than by calling anything.
//
// Two routes and not one, because the two models it serves are not the same
// model and the cheaper one is not always enough. Measured on chunks the free
// gateway had already answered, both are quicker than anything else the run
// has: the cheap model about seventeen seconds on a chunk of six thousand
// characters and the full one about forty, against forty to seventy on a box
// and five minutes of waiting when a box has quietly stopped. And on the chunk
// of chapter I, § 4 that three attempts on the free gateway could not get
// right, and which is dead on the queue because of it, the full model answered
// clean and the cheap one left \text{not } and \text{ or } standing in English.
//
// So the cheap one is ranked first and the full one behind it, which is the
// order the run wants anyway: the cheap model answers most chunks correctly and
// costs a fraction, and the chunks it cannot do are refused by the rules and
// asked again, and the model that answers them next is the one above it.
//
// CodexMini is a cut down model by its name, so quality.SmallModel says so and
// L08 reports any section it writes, which is the same treatment the cut down
// slug on the free gateway gets.
const (
	CodexCommand = "codex"
	CodexMini    = "gpt-5.4-mini"
	CodexFull    = "gpt-5.4"
)

// Key is the credential to send, preferring a literal one from the command line
// over the environment variable the route file names.
func (r Route) Key() string {
	if key := strings.TrimSpace(r.APIKey); key != "" {
		return key
	}
	if r.APIKeyEnv != "" {
		return strings.TrimSpace(os.Getenv(r.APIKeyEnv))
	}
	return ""
}

// Endpoint joins the base URL to a path, tolerating a base that already ends at
// /v1 and one that stops at the server root.
func (r Route) Endpoint(path string) string {
	base := strings.TrimRight(r.BaseURL, "/")
	if strings.HasSuffix(base, "/v1") {
		return base + path
	}
	return base + "/v1" + path
}

// UserAgent identifies this tool in the fleet's logs.
const UserAgent = "bourbaki-router"

// Client builds the transport for a route.
func (r Route) Client(timeout time.Duration, maxRetries int) (api.Completer, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if r.Timeout > 0 {
		timeout = r.Timeout.Duration()
	}
	if timeout <= 0 {
		// A page of Bourbaki through a browser session takes minutes. Anything
		// resembling an HTTP default would cut every call short.
		timeout = 20 * time.Minute
	}
	return &api.Client{
		URL:           r.Endpoint("/chat/completions"),
		APIKey:        r.Key(),
		HTTPClient:    &http.Client{Timeout: timeout},
		MaxRetries:    maxRetries,
		MaxRetryDelay: MaxRetryDelay,
		UserAgent:     UserAgent,
	}, nil
}

// MaxRetryDelay is the longest wait this fleet will sit out because a provider
// asked it to.
//
// The client has always honoured Retry-After and has always had the guard for a
// wait too long to be worth taking, and nothing set the guard. The free gateway
// answers a spent allowance with 429 and "retry-after: 53931", which is fifteen
// hours, so both lanes of a translation run went to sleep inside one call and
// stayed there. Ninety four minutes in, the log's last line was still the
// chunk that had come back before it, and nothing said why.
//
// A minute. A provider that names longer than that is telling us it is done for
// now, and the answer to that is the next route rather than a wait: the chunk
// fails this attempt, goes back on the queue, and the run picks it up on a
// route that is still answering. That is what the four free models are for.
const MaxRetryDelay = time.Minute
