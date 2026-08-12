package publish

import (
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"
)

// Search is spec 12 §6: client side, no server, one prebuilt index.
//
// One file per language rather than one file for the site. The whole English
// corpus is about a megabyte of prose once the mathematics is down to its
// source, and three languages in one file would be three megabytes fetched to
// search one of them. Which language a reader is searching is a thing the page
// already knows, so it fetches that one and nothing else.
//
// The index is generated here and is not committed, like the rest of the site.

// Entry is one searchable thing: a tagged statement or an exercise.
//
// The keys are one letter because there are two thousand of these in a file
// that goes over the wire, and a JSON key repeated two thousand times is the
// one place in this project where the length of a name is worth counting.
type Entry struct {
	Tag     string `json:"t,omitempty"`
	Label   string `json:"l"`
	Heading string `json:"h"`
	Section string `json:"s"`
	URL     string `json:"u"`
	Text    string `json:"x"`
}

// searchIndex is the entries of one language, in a total order so that two
// builds of the same corpus produce the same bytes.
func (s *Site) searchIndex(lang string) []Entry {
	var out []Entry
	// Over the statements rather than over the sections, because a tag names one
	// statement and the index is keyed on the tag. Walking the files instead would
	// put a statement in twice if two files ever carried the same tag, which is a
	// thing the build already refuses, and this way it cannot arise at all.
	for _, st := range s.Statements {
		sec := st.Sections[lang]
		if sec == nil {
			continue
		}
		u := st.Units[lang]
		// The URL is the tag page, because that is the page the tag scheme
		// promises and the one worth landing on from a search.
		out = append(out, Entry{Tag: st.Tag, Label: st.Label,
			Heading: plain(u.Heading.Text), Section: heading(sec),
			URL: s.TagURL(st.Tag), Text: plain(u.Body)})
	}
	for _, ex := range s.Exercises {
		if ex.Lang != lang {
			continue
		}
		// The exercise's own URL and not its tag URL, since the two serve the
		// same page and the exercise one is the canonical of the pair.
		sec := s.sectionAt(ex.Lang, ex.Meta.Book, ex.Meta.Chapter, ex.Dir)
		in := ""
		if sec != nil {
			in = heading(sec)
		}
		out = append(out, Entry{Tag: ex.Meta.Tag, Label: ex.Meta.Label,
			Heading: ex.Name(), Section: in, URL: s.ExerciseURL(ex), Text: plain(ex.Body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// searchJSON is the file one language's index is written as.
func (s *Site) searchJSON(lang string) (string, error) {
	b, err := json.Marshal(s.searchIndex(lang))
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// mathRE finds a display or an inline span. Display first, so that $$ is not
// read as an empty inline span.
var mathRE = regexp.MustCompile(`(?s)\$\$(.*?)\$\$|\$(.*?)\$`)

var (
	attrRE   = regexp.MustCompile(`\s*\{#[a-z0-9-]+(?:\s+[^}]*)?\}`)
	imageRE  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	markRE   = regexp.MustCompile("[*_`>|]+")
	bulletRE = regexp.MustCompile(`(?m)^\s{0,3}(?:[-+]|\d+\.)\s+`)
	spaceRE  = regexp.MustCompile(`\s+`)
)

// plain is the text of a body, for searching in.
//
// The mathematics is kept as its TeX source rather than dropped, which spec 12
// §6 asks for and which is right for a corpus where half the sentences are
// half formula: a reader looking for the statement about $\mathrm{Tr}$ has the
// backslash and the word both, and dropping the formulae would leave a great
// many statements with nothing searchable in them at all.
func plain(md string) string {
	var b strings.Builder
	last := 0
	for _, m := range mathRE.FindAllStringSubmatchIndex(md, -1) {
		b.WriteString(prose(md[last:m[0]]))
		// One of the two groups matched, and the other is -1.
		if m[2] >= 0 {
			b.WriteString(" " + md[m[2]:m[3]] + " ")
		} else if m[4] >= 0 {
			b.WriteString(" " + md[m[4]:m[5]] + " ")
		}
		last = m[1]
	}
	b.WriteString(prose(md[last:]))
	return strings.TrimSpace(spaceRE.ReplaceAllString(b.String(), " "))
}

// prose is the same for a run with no mathematics in it. It is separate because
// every rule here would eat a formula: an underscore is a subscript, a star is a
// product, and a pipe is a norm.
func prose(s string) string {
	s = attrRE.ReplaceAllString(s, "")
	s = imageRE.ReplaceAllString(s, " ")
	// linkRE is markdown.go's, since a link is a link in both places.
	s = linkRE.ReplaceAllString(s, "$1")
	s = bulletRE.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "#", " ")
	return markRE.ReplaceAllString(s, "")
}

// searchPage is /search/. It is the one page of the site with a script on it,
// and the script does one thing: fetch an index and filter it.
func (s *Site) searchPage() (string, error) {
	var b strings.Builder
	b.WriteString("<h1>Search</h1>\n")
	fmt.Fprintf(&b, "<form id=\"f\" action=%q method=\"get\">\n", s.url("search"))
	b.WriteString("<input id=\"q\" name=\"q\" type=\"search\" autofocus placeholder=\"a word, or a tag\">\n")
	b.WriteString("<select id=\"lang\" name=\"lang\">\n")
	for _, lang := range s.Langs {
		note := ""
		if s.Draft[lang] {
			note = " (partial)"
		}
		fmt.Fprintf(&b, "<option value=%q>%s%s</option>\n", lang, lang, note)
	}
	b.WriteString("</select>\n<button type=\"submit\">Search</button>\n</form>\n")
	b.WriteString("<p id=\"status\" class=\"none\">Searching runs in the browser over an index of the " +
		"statements and the exercises. Nothing is sent anywhere.</p>\n")
	b.WriteString("<noscript><p class=\"warn\">This page is the one part of the site that needs a script. " +
		"Everything else, the statements, the tags and the exercises, is plain HTML and works without one.</p></noscript>\n")
	b.WriteString("<ol id=\"r\" class=\"results\"></ol>\n")
	fmt.Fprintf(&b, "<script>\nconst BASE = %q;\n%s</script>\n", s.url("search"), searchJS)
	return s.render(layout{Title: "Search", Content: template.HTML(b.String())})
}

// searchJS is the whole of the client. No dependency, no build step, and no
// minifier: it is a hundred lines that anybody can read in the page source,
// which is the right size for a feature that ranks by counting words.
const searchJS = `
const q = document.getElementById('q'), sel = document.getElementById('lang');
const status = document.getElementById('status'), results = document.getElementById('r');
const cache = {};
let timer = null;

// The query is in the URL so a search can be linked to, which is what somebody
// does with a search that found the thing they will want again.
const params = new URLSearchParams(location.search);
if (params.get('q')) q.value = params.get('q');
if (params.get('lang')) sel.value = params.get('lang');

async function index(lang) {
  if (cache[lang]) return cache[lang];
  status.textContent = 'Loading the ' + lang + ' index.';
  const res = await fetch(BASE + lang + '.json');
  if (!res.ok) { status.textContent = 'The ' + lang + ' index did not load.'; return []; }
  cache[lang] = await res.json();
  return cache[lang];
}

function terms(s) {
  return s.toLowerCase().split(/[^\p{L}\p{N}\\]+/u).filter(t => t.length > 0);
}

function count(hay, needle) {
  let n = 0, i = hay.indexOf(needle);
  while (i >= 0) { n++; i = hay.indexOf(needle, i + needle.length); }
  return n;
}

// Term frequency, with a hit in the heading worth more than one in the body.
// Spec 12 section 6: ranking beyond this is not worth building for a thousand
// statements, and a reader who knows what they are looking for types the tag.
function score(e, ts) {
  // The tag and the label are part of what is searched and not only of what is
  // shown. Typing a tag is the fastest way to a statement anybody has, and a tag
  // is in none of the prose, so searching the prose alone answers 00QO with
  // nothing at all.
  const head = (e.h + ' ' + e.s + ' ' + e.t + ' ' + e.l).toLowerCase(), body = e.x.toLowerCase();
  let total = 0;
  for (const t of ts) {
    const inHead = count(head, t), inBody = count(body, t);
    if (inHead + inBody === 0) return 0;
    total += inHead * 8 + inBody;
  }
  if (e.t && ts.length === 1 && e.t.toLowerCase() === ts[0]) total += 1000;
  return total;
}

function snippet(text, ts) {
  const low = text.toLowerCase();
  let at = -1;
  for (const t of ts) { const i = low.indexOf(t); if (i >= 0 && (at < 0 || i < at)) at = i; }
  if (at < 0) at = 0;
  const from = Math.max(0, at - 60);
  return (from > 0 ? '…' : '') + text.slice(from, from + 260) + (text.length > from + 260 ? '…' : '');
}

function escape(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

async function run() {
  const text = q.value.trim();
  results.innerHTML = '';
  if (!text) { status.textContent = 'Type a word, or a tag such as 00QO.'; return; }
  const ts = terms(text);
  const entries = await index(sel.value);
  const hits = [];
  for (const e of entries) { const n = score(e, ts); if (n > 0) hits.push([n, e]); }
  hits.sort((a, b) => b[0] - a[0] || a[1].l.localeCompare(b[1].l));
  status.textContent = hits.length + (hits.length === 1 ? ' result' : ' results') +
    (hits.length > 50 ? ', the first 50 shown' : '') + '.';
  for (const [, e] of hits.slice(0, 50)) {
    const li = document.createElement('li');
    li.innerHTML = '<a href="' + e.u + '">' + escape(e.h) + '</a>' +
      (e.s ? ' <span class="count">' + escape(e.s) + '</span>' : '') +
      (e.t ? ' <code class="count">' + e.t + '</code>' : '') +
      '<p class="snippet">' + escape(snippet(e.x, ts)) + '</p>';
    results.appendChild(li);
  }
}

function later() { clearTimeout(timer); timer = setTimeout(run, 120); }
q.addEventListener('input', later);
sel.addEventListener('change', run);
// The form works without a script as a link back to this page with the query in
// it, and with one it should not reload the page to do the same search again.
document.getElementById('f').addEventListener('submit', e => { e.preventDefault(); run(); });
if (q.value) run(); else status.textContent = 'Type a word, or a tag such as 00QO.';
`
