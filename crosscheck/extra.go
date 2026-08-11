package crosscheck

import "strings"

// Extra is the other direction: the words our reading has and pdftotext has
// not.
//
// Page asks which of pdftotext's words we lost, and a page can pass that and
// still be wrong. An accent drawn over the letters of the line below left page
// 114 reading "the assumpti~ons", and "assumptions" is further down that same
// page, spelled right, so the bag of words had it and nothing was reported.
// What that page has and pdftotext has not is "assumpti", and that is what this
// looks for. Run over the volume before the accents were fixed it names pages
// 113, 114, 354, 424, 429 and 466, which is every page the accents landed on.
//
// The excuses of the other direction are not repeated here, and that is the
// point rather than an omission. There a word of ours running on to its
// neighbour was pdftotext gluing a formula to a word; here it is us breaking a
// word that pdftotext kept whole, which is the defect itself.
func Extra(ours, theirs string) []Lost {
	// Their reading has to be put back together first. pdftotext leaves the
	// typesetter's hyphens where they fall, so the long words are the ones it
	// breaks, and asking a raw page for "homomorphisms" is asking for a word it
	// prints as "homo-" and "morphisms".
	said := set(strings.Join(flow(theirs, false), "\n"))
	var out []Lost
	seen := map[string]bool{}
	for _, line := range strings.Split(ours, "\n") {
		for _, w := range words(strip(line)) {
			if said[strings.ToLower(w)] {
				continue
			}
			// A compound word broken at its own hyphen. The volume sets
			// "k-algebras" with the ring in mathematics and breaks the line
			// after the hyphen, and putting that back together leaves
			// pdftotext with "kalgebras" and no "algebras" on the page at all.
			if said[strings.ToLower(strings.ReplaceAll(w, "-", ""))] {
				continue
			}
			for _, p := range strings.Split(w, "-") {
				key := strings.ToLower(p)
				if !candidate(p) || said[key] || seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, Lost{Word: p, Line: strings.TrimSpace(line), Ours: true})
			}
		}
	}
	return out
}
