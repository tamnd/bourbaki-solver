// Package katex renders TeX to HTML at build time, with no Node and no
// network, by running the real katex.min.js inside a JavaScript engine.
//
// Rendering in the browser was the other option and it is the wrong one for
// this corpus. The Markdown is 4.4 MB with mathematics in most paragraphs, so
// browser rendering means shipping the engine as well as the fonts and then
// showing every reader a flash of raw TeX on a site whose whole content is
// mathematics. Rendering here means the HTML holds the finished markup and the
// browser needs the stylesheet and the fonts and nothing else.
//
// Three ways to reach KaTeX from Go. Shelling out to Node puts a second
// toolchain in CI and on the laptop for a project that is otherwise Go and
// poppler. A Go translator that handles what Bourbaki writes does not exist,
// and writing one is not a trade worth making. Embedding a JavaScript engine
// and running the real file costs one vendored dependency and keeps the build
// pure Go, which is what this does.
//
// The file is vendored rather than fetched, and its SHA-256 is recorded in
// SHA256SUMS and checked by a test, for the same reason every source PDF has
// its hash written down: a dependency that can change under us without saying
// so is not a dependency, it is a risk.
package katex

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

// Version is the KaTeX release vendored here. It is written down because the
// error messages below are its error messages and they move between releases.
const Version = "0.18.4"

//go:embed katex.min.js
var script string

//go:embed assets
var assetsFS embed.FS

// Assets is the stylesheet and the fonts a page needs to display what Render
// writes: assets/katex.min.css and assets/fonts/*.woff2.
//
// Only woff2 is vendored, of the three formats KaTeX ships. The stylesheet
// names all three in one src list and every browser that has shipped since 2016
// takes the first it understands, so the other two are bytes nobody would ever
// fetch.
func Assets() fs.FS {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err) // the directory is embedded above; this cannot fail at run time
	}
	return sub
}

// Renderer holds one JavaScript engine with KaTeX loaded in it.
//
// Loading the script costs about a tenth of a second and rendering a span costs
// well under a millisecond, so the engine is built once and kept. A Renderer is
// safe to use from several goroutines: goja is not, and the lock is what makes
// up the difference.
type Renderer struct {
	mu    sync.Mutex
	vm    *goja.Runtime
	call  goja.Callable
	cache map[string]string
}

// New loads KaTeX into a fresh engine.
func New() (*Renderer, error) {
	vm := goja.New()
	if _, err := vm.RunString(script); err != nil {
		return nil, fmt.Errorf("loading katex %s: %w", Version, err)
	}
	fn, ok := goja.AssertFunction(vm.Get("katex").ToObject(vm).Get("renderToString"))
	if !ok {
		return nil, fmt.Errorf("katex %s has no renderToString", Version)
	}
	return &Renderer{vm: vm, call: fn, cache: map[string]string{}}, nil
}

// Render returns the HTML for one span of TeX.
//
// An error is a refusal by KaTeX and it is returned rather than swallowed. A
// span that does not parse is a fault in the extraction, and falling back to
// printing the raw TeX would put the fault on the page in a form that looks
// deliberate. The message is KaTeX's own, which names the character it stopped
// at.
func (r *Renderer) Render(tex string, display bool) (string, error) {
	return r.render(tex, display, "")
}

// MathML renders one span of TeX as MathML and nothing else.
//
// The site wants what Render gives, which is MathML for a screen reader with a
// pile of positioned spans over the top of it for the eye, because a browser
// with the KaTeX stylesheet and the KaTeX fonts sets that beautifully and a
// browser is what the site is read in. An EPUB is not read in a browser. It is
// read in a dozen reading systems with a dozen ideas of how much CSS they will
// honour, and the ones that honour least would show the positioned spans with
// no positioning, which is every symbol of a formula in a row at the same size
// with the fractions inside out.
//
// So the book takes the other half of the same render. MathML is what EPUB 3
// requires a reading system to support, it needs no stylesheet and no font file,
// and it is the same parse of the same TeX by the same engine, so a formula that
// is right on the site is right in the EPUB or neither is.
func (r *Renderer) MathML(tex string, display bool) (string, error) {
	return r.render(tex, display, "mathml")
}

func (r *Renderer) render(tex string, display bool, output string) (string, error) {
	key := output + "\x00" + tex
	if display {
		key = output + "\x00$$" + tex
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if html, ok := r.cache[key]; ok {
		return html, nil
	}

	opts := r.vm.NewObject()
	opts.Set("displayMode", display)
	if output != "" {
		opts.Set("output", output)
	}
	// throwOnError is the default and is set anyway, because the alternative is
	// KaTeX writing the error into the page in red, which is the one behaviour
	// this must not have.
	opts.Set("throwOnError", true)
	// No macros. Bourbaki writes \mathscr, \mathfrak and \mathbf across the
	// chapter and KaTeX knows all three.
	//
	// The corpus used to write \dbend here, the dangerous bend the book prints
	// in the margin of a hard passage, out of a LaTeX package nobody here
	// loads. It was deliberately never defined: the 17 spans were the bare
	// command with no mathematics around it, which is not a formula at all, and
	// the answer was to write the sign in the Markdown rather than to teach the
	// renderer a command the corpus should not be emitting. Extraction writes
	// U+2621 now and this stays empty.
	opts.Set("macros", r.vm.NewObject())
	opts.Set("strict", false)

	v, err := r.call(goja.Undefined(), r.vm.ToValue(tex), opts)
	if err != nil {
		return "", errors.New(clean(err.Error()))
	}
	html := v.String()
	r.cache[key] = html
	return html, nil
}

// clean takes the engine's position off the end of a KaTeX error. The position
// is inside the one line of JavaScript this package evaluates and says nothing
// about the corpus, and the caller knows the file and the line that matter.
func clean(msg string) string {
	if i := strings.Index(msg, " at <eval>:"); i > 0 {
		msg = msg[:i]
	}
	msg = strings.TrimPrefix(msg, "ParseError: ")
	return strings.TrimPrefix(msg, "KaTeX parse error: ")
}
