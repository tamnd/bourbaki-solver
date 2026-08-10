// Package bourbaki builds a tagged, multilingual Markdown corpus from the
// PDFs of Bourbaki's Éléments de mathématique.
//
// The pipeline is: extract each page, either natively for the born-digital
// volume or through vision OCR for the scans, assemble pages into sections,
// split out exercises, assign permanent tags, resolve cross-references,
// translate, and solve.
//
// The subpackages carry the work. This root package holds only the version
// string, so that importers can depend on it without pulling in poppler or SSH.
package bourbaki

// Version is set at build time with -ldflags "-X github.com/tamnd/bourbaki-solver.Version=...".
var Version = "dev"
