This image is one line cut out of a page of a printed mathematics book. Transcribe that line and nothing else.

- Output the line as a single line of Markdown with LaTeX, with no preamble, no closing remark, no code fence and no explanation.
- The cut is tight, so the descenders of the line above and the ascenders of the line below can show at the top and bottom edges. Transcribe only the complete line that runs across the middle of the image. Ignore anything sliced off at an edge.
- Copy every word, symbol and number exactly as printed, in the language it is printed in, and translate nothing.
- Write mathematics as LaTeX between single dollar signs. Prose stays prose: a sentence that happens to name a variable gets dollar signs around the variable and not around the sentence.
- A word broken at the end of the line keeps its hyphen exactly as printed. Do not complete it and do not join it to anything.
- Accents are drawn over the letters they belong to and the letters they cover are part of what is being read. Write \hat, \check, \bar, \tilde, \dot, \breve, \mathring for a mark over one letter, and \widehat, \widecheck, \widetilde, \overline when the mark is drawn wide enough to span several letters or a whole expression: $\widehat{G/H}$ is not $\widehat{G}/H$, and the width of the mark on the page is what says which one it is.
- An accent that is part of a French or German word is part of the word: écrit, algèbre, Hölder, Šmulian. Write those as letters and not as LaTeX.

The line comes from Bourbaki, Elements of Mathematics. The following hold for these volumes.

- Bourbaki sets the standard rings and fields in bold, not blackboard bold. Write \mathbf{Z}, \mathbf{Q}, \mathbf{R}, \mathbf{C}, \mathbf{N}, \mathbf{F}_q. Never write \mathbb{Z} or any other \mathbb.
- Script and fraktur letters are kept as they are set: \mathfrak{g} for a Lie algebra, \mathcal{F} or \mathscr{F} for a sheaf or a filter.
- Cross-references are transcribed verbatim, with their punctuation and spacing: (I, p. 23, Proposition 4), (Set Theory, III, § 3, no. 6, Proposition 13).
- Keep § and no. in that form. Do not expand them to Section or number.
- A statement head is printed in small capitals and followed by a spaced em rule. Transcribe it in bold with the rule kept, as in **Proposition 4.** —.
- If part of the line is illegible, write ⟪illegible⟫ at that point rather than guessing. Do not invent a subscript, an index or a digit you cannot read.
