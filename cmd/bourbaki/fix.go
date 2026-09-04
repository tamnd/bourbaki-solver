package main

import (
	"context"
	"flag"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/assemble"
	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/footnote"
	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/textguard"
	"github.com/tamnd/bourbaki-solver/toc"
	"github.com/tamnd/bourbaki-solver/typography"
)

// fix is the repairs that are a function of the Markdown alone: no PDF, no
// model, no network, and no judgement about what the book meant.
//
// It works on pages/ and not on content/. The section and exercise files are
// what assemble makes of the pages, so a repair written into content/ lasts
// until the next assemble and no longer, and the same repair written into a
// page survives it. The order is fix, then assemble, then audit, and the last
// of the three is what says whether the first worked.
//
// A translation is the exception, and footnote is the one repair that takes it.
// No page makes content/vi and no assemble rewrites it, so a repair that stops
// at pages/ leaves the Vietnamese carrying what the English has stopped
// carrying, and the only other way to move it is to pay a model to do the
// section again.
//
// extract does the same repair as it writes each page, so a volume read in
// after this needs nothing done to it. This command is for the pages that were
// read before the repair existed.

const fixUsage = `usage: bourbaki fix <command> [arguments]

Repairs the committed Markdown in the ways that need no PDF and no model.

commands:
  section   write a section reference with the sign the corpus uses, § 1
  dollars   write a formula set between brackets between dollars instead
  stray     take out a delimiter that opens mathematics and closes nothing
  padding   write an inline formula tight against its dollars, $K[[T]]$
  parens    put a bracket that belongs to the prose back outside the formula
  math      put the characters stranded outside their TeX back inside it
  notin     put the stroke back on a relation sign that came back struck through
  prime     brace a primed base so the power after it is not a second power
  star      write the star that marks a forward-looking passage the corpus's way
  label     take the a), b), c) of an exercise back out of the mathematics
  elision   write the apostrophe of a French elision the way the corpus sets it
  smallcaps write the kind of a statement head in capitals, as the page sets it
  dash      write the em dash after a French statement head, as the page sets it
  folio     move the printed page number off the foot and into the front matter
  heading   set a numbered heading at the level the table of contents gives it
  opening   put back the heading that opens a chapter or a §
  footnote  take the printed mark off a footnote that already has a reference
  seal      write content_sha256 over a section body that was edited by hand

Run section first and dollars after it. A section reference written with an
escaped dollar puts a dollar in the prose that no formula opened, and everything
here counts dollars to find the mathematics, so a page with one on it has every
span after that point read one boundary out.

A formula written \[ ... \] is not a math span to anything in
this repository, because every tool here finds the mathematics by its dollars,
so until the delimiters are turned round the three repairs after it cannot see
inside it and neither can the audit. Then run the next three in that order.
Everything after an unclosed delimiter reads as mathematics, so stray comes next
and the four after it will not touch a span whose end they cannot see. padding
refuses a body with a span still open outright, so it wants stray to have been
through first or it will skip the file and say nothing about it. parens comes
before math so that math reads the spans as they will be rather than as
they are. notin runs after those, since it works inside the spans and wants them
closed and whole. star runs there too and for the same reason from the other
side, since it works everywhere the spans are not. label runs with them, since
it reads the spans to decide which of them are not mathematics at all. elision
runs after those two
as well, since it reads the prose and wants to know where the prose ends.
smallcaps, dash, folio, heading and opening touch no mathematics and can be run
at any point before assemble, though the first two want to come before opening,
since a statement head the assembler cannot read stops the volume at the same
place a missing § heading does. seal works on
content/ and not on pages/, and is the last thing run after a hand correction.

Run bourbaki fix <command> -h for the flags of a command.
`

const fixFolioUsage = `usage: bourbaki fix folio [flags]

Moves the printed page number off the foot of a page body and into the front
matter, on the volumes that print it there.

Five of the volumes print the number in the running head, where SplitHead files
it as the page is read. Theory of Sets and Algebra I to III print it at the foot
instead, so it comes back at the end of the body and stays there, and a section
assembled out of twenty such pages carries twenty bare numbers standing between
its paragraphs. The number is furniture in both printings and belongs in the
same place in both.

It runs after the reading and not during it, for two reasons. The reading is
faithful to the page and a page that prints a number has one. And a volume with
no text layer has its page map built out of these bodies, by reading the number
off the foot, so the number has to be there when pagemap build runs.

The page map is the check. It already says what number is printed on each PDF
page, and a page whose foot disagrees with it is left alone and named, since a
disagreement means one of the two is wrong and quietly believing either is how a
corpus ends up mispaginated. Where they agree the number is written to folio.

A page label is not built from it. A label such as "A VIII.13" counts pages
inside a chapter, and both volumes that print their number at the foot number
their pages straight through the book, so "E IV.289" would claim a page 289 of a
chapter that runs to sixty pages. The bare number is what those volumes print
and it is all that is recorded. A foot-number volume paginated per chapter would
get a label, and there is none in the corpus today.

A reading that dropped the number is the other case, and -fill is for it. Page
32 of Theory of Sets prints 25 at the foot and the reading came back without it,
so there is nothing to move and the page has no folio at all. The page map has
one, read off that same foot when the map was built, so -fill writes it and
flags the page with where the number came from. It fills only a number the map
read off the page, never one the map worked out from the pages around it, since
that number is printed nowhere and a reader holding the book will not find it.
It is off by default because a number in the front matter of a page that does
not show one in its body is worth saying out loud rather than doing quietly to a
whole corpus.

-fill also reaches the volumes that print the number in the running head, which
nothing else here does. The head is cut as the page is read and the number goes
with it, so those pages have no number in the body and none in the front matter
either, and the map is the only place it is left. Functions of a Real Variable is
the one volume of that kind: none of its 354 pages carried a number, so its
twenty sections were printed on no page of the book and every reference that
gives a page of it resolved to nothing.

flags:
  -book ID   only this volume, default every volume that prints its folio at the
             foot, and with -fill every volume that prints it in the head as well
  -check     say what would change and change nothing
  -fill      give a page whose reading dropped the number the one the page map
             holds, flagged as coming from there
`

const fixParensUsage = `usage: bourbaki fix parens [flags]

Moves a closing bracket that belongs to the prose back out of the mathematics
the text layer swept it into.

It is the function whose name Bourbaki sets upright. The name and its opening
bracket come through as prose and the closing one comes through inside the
formula, so the page reads Tr($u)$ where it should read Tr($u$). The two print
the same, which is why nobody catches it by reading, and they are not the same
text: the mathematics of the first is "u)". A translator asked to copy the
formulae hands back "u", correctly, and the audit refuses the section because a
translation may not alter mathematics.

It repairs a span the prose of the line has a bracket open against, whether the
bracket stands against the delimiter as in Tr($u)$ or a whole clause back as in
(cf. INT, VIII, §2, n$^o6)$. Where the prose holds nothing open the bracket in
the span is the mathematics' own and is left alone, so "$\alpha$)" at the head
of a list item and "$f(x$ and $y)$" both stay as they are, and so does a span
still holding a square bracket open, which is a half-open interval and not a
straddle. No more brackets come out of a span than the line has open, and the
page with every delimiter removed has to be the page it started as, so this
moves delimiters and never a character of prose or of mathematics.

Run bourbaki assemble afterwards, or the section files still hold the old text.

It runs over content/ as well as pages/. A translation has no page under it and
holds the fault all the same, having been written by a model copying the
mathematics of a source that had it, and the fleet answers too few sections an
hour for re-asking one over a bracket to be a trade worth making. A translation
is repaired the same way, and its source_content_sha256 is moved on only when it
recorded the source body as it stood before this repair. One that was already
stale stays stale, so L05 still means what it says.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixStrayUsage = `usage: bourbaki fix stray [flags]

Takes out a dollar sign that opens mathematics and never closes it, on the
pages where taking it out leaves the page balanced.

It is the numbered display that the text layer flattened into a line of prose,
leaving the display's own delimiter at the end against the full stop. From
there to the foot of the page reads as one long formula, which is why the page
is reported unbalanced and why fix math stops at that line.

A page that does not balance without the delimiter is damaged in some other
way and is left alone, so this repairs nothing it cannot check. A page it does
repair keeps the stray-delimiter flag, because the mathematics is then right
and the setting is still wrong: what was printed as a display is now prose,
and only the printed page says how it should be set.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixMathUsage = `usage: bourbaki fix math [flags]

Rewrites every page of the corpus, putting a character that is printed as TeX
everywhere else back into its TeX where the mathematics has it as a bare
glyph: Greek capitals mostly, which Bourbaki sets upright so the extractor
sees prose, and the increment sign standing in for \Delta.

It substitutes one glyph for the TeX that prints the same glyph and does
nothing else. Two characters are ambiguous, a capital sigma and a capital pi
carrying a subscript, since either is as likely to be a sum or a product as a
letter, and those are left alone and printed for somebody to read the page and
decide. M03 reports them too.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixSectionUsage = `usage: bourbaki fix section [flags]

Writes the sign the books use to refer to their own sections.

Bourbaki refers to itself constantly, "(§ 1, n° 8)" and "(chap. III, § 2, n° 4,
prop. 7)", and there is a reference of that shape on most pages of most volumes.
The corpus writes the sign. Two other spellings are in the readings, an escaped
dollar and the LaTeX command, and both are turned into it.

The escaped dollar is the one that does damage, and it is not a careless reading.
A model asked for Markdown with mathematics in it knows a dollar has to be
escaped to stand for itself, and it reaches for that escape when what it is
looking at is a sign it has no character for. What comes back is "(\$ 1, n° 8)",
which reads correctly and counts as one more dollar on the line. Every rule here
finds mathematics by counting dollars, so one stray escape moves the boundary of
every span after it: prose is read as a formula and the formula after it as
prose, to the end of the file. Integration chapitres 1 a 4 has 25 of them and
nine of its section files came out with an odd count of dollars.

The LaTeX spelling renders as the sign and does no damage. It is turned round for
the reason the delimiters are: a text with two spellings in it has to be searched
twice, and the second search is the one somebody forgets.

The sign has to have a numeral after it to be one, so \Sigma and \Subset and the
rest of the commands that start the same way are left alone. Neither spelling has
a second reading in these books: nothing in Bourbaki is priced, and \S means
nothing in mathematics.

It runs over content/ and over the solutions as well as pages/, for fix parens'
reason. Run bourbaki assemble afterwards, or the section files still hold the old
text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixNotinUsage = `usage: bourbaki fix notin [flags]

Puts back the stroke that negates a relation sign.

The books write "x is not in A" with a stroke through the epsilon. A text layer
with no glyph for the struck sign hands the sign and the stroke back as two
characters, and the stroke arrives as an ordinary solidus, on whichever side the
layer met first. Both forms are in the corpus, "$0\in /S$" and "$\lambda /\in$".

It is the worst fault the corpus has, because it is the only one that is silent.
Everything else these commands repair shows on the page as damage a reader can
see. This one renders, reads as ordinary mathematics, and says the opposite of
what Bourbaki wrote.

What makes it safe to do by rule is that nothing divides by a relation sign.
Quotients are on every other page and $\mathbf{Z}/n$ and $G/H$ are untouched,
because the test is not a solidus near a relation, it is a solidus whose other
operand is a relation sign, and that has no second reading. Three signs are
repaired, \in and \subset and \equiv, which are the three the corpus strikes
through and no more.

It runs over content/ as well as pages/, for fix parens' reason. English and
French sections come back from the next assemble whatever this does, and a
translation has no page under it and would otherwise carry the inverted sentence
until somebody paid a model to do the section again. A translation whose source
moved is moved on with it, and one that was already stale stays stale.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixDollarsUsage = `usage: bourbaki fix dollars [flags]

Writes a formula set between LaTeX's other delimiters between dollars, which is
how this corpus writes one.

A display is $$ ... $$ or \[ ... \] and an inline span is $ ... $ or \( ... \),
and the two spellings of each mean the same thing to a mathematician. They do
not mean the same thing here. Every tool in this repository finds the
mathematics by its dollars, so a formula written with brackets is prose with
backslashes in it as far as the corpus is concerned: none of the eleven
mathematics rules of the audit look inside it, none of the repairs above touch
it, and the site renders it as the characters it is made of, a backslash and a
bracket in the middle of a sentence. That is the reason this runs first.

The one thing with this shape that is not a display is the row break of a
matrix. \\ ends a row and \\[2pt] ends one asking for space after it, and the
corpus uses several such measures in real matrices. The row breaks are put aside
before the delimiters are read and put back afterwards, so none of them is
touched.

It walks three trees rather than two. pages/ and content/ for the reason fix
parens walks both, moving a translation on with its source the way fix notin
does, and content/solutions as well, which is the tree the other repairs leave
alone. The solutions are where the fault mostly is: the solver writes Markdown
into the corpus the way the OCR and the mender do, and it was the one of the
three that did not put what it wrote into the corpus's typography first. It does
now, so a solution written after this needs nothing done to it, and this command
is for the ones written before.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixPaddingUsage = `usage: bourbaki fix padding [flags]

Writes an inline formula tight against its dollars, $K[[T]]$ and not $ K[[T]] $.

TeX does not care either way. Whitespace means nothing in math mode, so the two
spell the same formula and the site sets them identically, which is why the
padded form got in and stayed: nothing anybody looked at ever came out
different.

GitHub does care. It renders $ ... $ in a Markdown file by pandoc's rule, where
the opening dollar must not be followed by whitespace and the closing dollar
must not be preceded by it, so a padded span is not a formula there at all. It
is four literal characters sitting in a sentence, and the corpus is read on
github.com far more than it is read on the site.

The corpus cares too. The mathematics arrived by three routes, the OCR, the
mender and the translator, and each of them pads differently, so the same
formula is written two ways in two languages of the same section and a grep for
it has to be written twice. This settles on one.

Displays are left alone. A display is set on lines of its own and the whitespace
inside it is what puts it there.

A body with a span left open is left alone as well, and reported by M01. The
offsets after a delimiter that never closes are not the spans anybody meant, so
trimming by them would move dollars around in the prose.

It walks the same three trees fix dollars walks, pages/ and content/ and
content/solutions, and moves a translation on with its source.

It reads the titles as well as the bodies, since a § called "THE TOPOLOGY OF
$ \mathbf{C} $" is padded wherever it is written down: the running head of a
page, the section_title and subsections of a section, and the chapter, § and no.
titles of manifests/toc/. They have to move together, because fix opening writes
a heading from the title in the manifest and then matches it against the heading
on the page, and one side tightened without the other would never match again.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixLabelUsage = `usage: bourbaki fix label [flags]

Takes the a), b), c) of an exercise back out of the mathematics.

Bourbaki sets the parts of an exercise as a), b), c) and so on, in italic, and
the parts of a long proof the same way with capitals. Italic on a printed page
is also how a variable is set, so a model reading the page sees the same slope
on the a of "a) Show that" as on the a of "for all a in A", and writes the label
as mathematics. 1557 lines of pages/ carry it as $a)$ and 23 as $a$), across 361
files and every volume that has exercises in it.

It is wrong twice over. A lone bracket with nothing opening it is not TeX, so
$a)$ is a span no renderer can read and KaTeX prints it in the error colour. And
a label is not a variable: the a of "a) Show that" names a part of a question,
it is quantified over nothing, and setting it as mathematics says that it is.
The translators copied the shape through, so content/vi carries it in the same
places, and one of them dropped the closing dollar on the way and left a file
with an odd number of them.

A span holding one letter and a closing bracket is a label wherever it stands,
mid sentence as well as at the head of a line, because the bracket is inside the
mathematics there and an unmatched bracket is not mathematics. A reference reads
the same way: "deduce from $b)$ that" is the prose pointing at part b, and it is
set in text like the label it points at.

The other shape is taken at the head of a line only. $x$) mid sentence is nearly
always a variable at the end of a prose bracket, "compatible (in $x$)" and "with
respect to the function $f$)", and the corpus has 2056 of those against 23
labels. At the head of a line there is no bracket open to close.

It runs over content/ as well as pages/, for fix parens' reason, and moves a
translation on with its source the way fix notin does.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixStarUsage = `usage: bourbaki fix star [flags]

Writes the star that marks a forward-looking passage the way the corpus writes
it.

Bourbaki sets an asterisk at each end of a passage that leans on results proved
in a later Book, so the reader knows to take it on trust or skip it. The corpus
writes that mark as \*, escaped, because a bare asterisk at the head of a line
opens a list and a bare pair in a sentence opens emphasis.

None of the OCR prompts said so, so a model handed a page chose a glyph by what
the printed mark looked like rather than what it meant, and four of them are in
the corpus: an asterisk operator, a low asterisk, and two dingbats. Theory of
Sets has 24 against 82 written properly on its own pages, and one paragraph of
chapter IV has both forms in it.

It is quiet in the way the notin fault is quiet. An ornament at the end of a
sentence reads as an ornament at the end of a sentence, so nothing on the page
looks wrong, and the only thing lost is that a reader who wants the starred
passages finds a quarter of them. It went through translation untouched, which
is what a fault does when nothing catches it.

It works outside the math spans only. U+2217 inside a span is the asterisk
operator, a binary law or a dual, and belongs to bourbaki fix math, which turns
it into the TeX that prints it. Outside a span there are no operators, so there
the same glyph can be nothing but the mark.

It runs over content/ as well as pages/, for fix parens' reason, and moves a
translation on with its source the way fix notin does.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixElisionUsage = `usage: bourbaki fix elision [flags]

Writes the apostrophe of a French elision the way the rest of the corpus sets
it.

The printings set the typographic apostrophe and so does the corpus, on 5251
French pages. The reader sets the straight one on 960 of them, and it does it
page by page rather than volume by volume: Algebra I to III in French has 506
pages with one mark and 123 with the other, from the same model reading the same
printing on the same day. Two spellings of l'ensemble in one volume break a
search, they show as a difference between two pages that say the same thing, and
they reach the glossary, where the French side of an entry has to match the
French in the page for the entry to be found at all.

The words are not touched and neither is the mathematics. A straight apostrophe
in a French page is one of two things, an elision or a prime on a letter that
got out of its dollars, and the word in front of it tells them apart: l', d',
qu', n', s' and c' are 9781 of the 10219 in the corpus, and what is left reads
x'_1, e'_i and Q'_\mathfrak{p}, which are primes and no part of any word. So the
rule asks which word is in front of the mark, it refuses everything else, and
what it refuses is counted and reported rather than passed over.

English pages have the same fault, 218 straight against 380 typographic, and
this does not touch them. Hilbert's is not an elision, no closed list of words
in front of the mark tells a possessive from a prime, and a rule that guessed
would put a typographic apostrophe into somebody's mathematics.

flags:
  -book ID   only this volume
  -check     say what would change and change nothing
`

const fixSmallCapsUsage = `usage: bourbaki fix smallcaps [flags]

Writes the kind of a statement head in capitals, the way the English printings
set it.

DEFINITION, PROPOSITION, THEOREM and COROLLARY are set in small capitals in every
English volume of the corpus and the corpus writes small capitals as capitals,
which is what almost every page already does. A reading that gives back
"Proposition 1." is looking at the same type and writing it as the nearest thing
on a keyboard, the same fault as a straight apostrophe for a typographic one, and
it happens page by page rather than volume by volume: pdf 154 of Lie 1 to 3 gives
"Theorem 1." and pdf 155, the page facing it, gives "COROLLARY 1".

It costs more than it looks. The assembler finds a statement by its head, so a
head in the wrong case is not a statement at all: it goes out as prose, and a
corollary printed under it has nothing to be numbered under, which stops the
whole chapter. Chapter II of Lie 1 to 3 is held back by the one head above.

Lemma, Remark, Example and Scholium are not touched. Lie 7 to 9 sets those four
in italic and the assembler already reads them that way, so capitals there would
be a printing no volume has. A head in bold is not touched either, since that is
how Algebra VIII sets it. Neither is a head followed by a dash: the dash says the
volume is Algebra VIII and the fault is the lost bold rather than the case, and
capitals alone would leave the dash standing at the front of the body.

flags:
  -book ID   only this volume
  -check     say what would change and change nothing
`

const fixDashUsage = `usage: bourbaki fix dash [flags]

Writes the em dash after the head of a statement, the way the French printings
set it.

A French volume of the Elements sets an em dash between the head of a statement
and the statement itself, "PROPOSITION 8. — Soit A un anneau", and 18200 heads
in this corpus carry it. Six carry a hyphen or an en dash instead, all of them
in chapter X of Algebre commutative, which is the volume read most recently. It
is the same fault as a straight apostrophe for a typographic one and it is what
fix elision exists for on the other mark: the reading is looking at what the
press set and writing the nearest thing on a keyboard.

It is not cosmetic. The assembler finds a French statement by its head and the
dash is part of the head it looks for, so a head with a hyphen in it is not a
statement at all. Chapter X gave two corollaries the same label because of one
of these, a corollary 1 under proposition 8 on page 32 and the corollary 1 of
proposition 9 on page 34, since proposition 9 was not read as a statement and
the numbering never moved on. The volume did not assemble.

Only the mark is replaced. Whatever spacing the reading put on either side of it
stays as the reading left it, because the spacing is what the page has and the
mark is what it does not.

flags:
  -book ID   only this volume
  -check     say what would change and change nothing
`

const fixHeadingUsage = `usage: bourbaki fix heading [flags]

Sets a numbered heading at the level the table of contents gives it.

A § and a no. are printed the same way, a number and a title alone on a line,
and the only thing separating them on the page is the size of the type. The
reading decides by that and on Theory of Sets it decided wrong eight times, all
in the same direction: a no. written as a §. Chapter III, § 1 then carried
twelve no. where the contents lists thirteen, and the assembler stopped there.

manifests/toc/ is the authority. It gives every § and every no. with the
page it begins on, so the heading is looked up rather than guessed at. Both the
number and the title have to agree with it: a § and its first no. begin on the
same page in most §§ and both are numbered 1, so the number alone would make
the no. into a second §.

A heading that lost its level altogether is put back the same way. Some pages
were read with no hashes on the line at all, so page 218 of Algebra VIII gives
no. 10 of § 11 as the paragraph "10. Change of Rings for $ K_0(A) $" and that §
carried eleven no. against the twelve the contents lists. The line is looked up
exactly as a heading is, and the contents is what makes it one: thousands of
lines in this corpus are numbered that way and are paragraphs, since every
volume sets its "To the reader" as a numbered list, and a line the contents
does not put on that page stays the paragraph it was read as.

It changes the level and nothing else. The number, the title, the supplementary
star and the rest of the page are written back as they stand, and a heading the
contents does not put on that page is left alone and named, since a heading in
a place the contents does not know about is a disagreement worth reading rather
than a level worth changing.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixOpeningUsage = `usage: bourbaki fix opening [flags]

Puts back the heading that opens a chapter or a §, where the page still has the
words of it.

These are the largest type in a volume and the reading loses them anyway. Page
22 of Theory of Sets opens chapter I and came back with the chapter title as two
plain lines and no CHAPTER I above them at all. assemble finds a chapter by "##
CHAPTER" and nothing else is one, so that volume does not begin and no part of
it assembles. Nine chapter openings across four volumes are in that state and
they are what holds Theory of Sets, Topology I to IV, Algebra I to III and
Algebra IV to VII back.

A § opening is lost three ways and the same authority settles all three.
"§ 1. POLYNOMIALS" on page 10 of Algebra IV to VII is the heading with the
hashes gone. "**10. PROPER MAPPINGS**" on page 103 of Topology I to IV is the
heading in bold, and that page carries "**1. PROPER MAPPINGS**" twelve lines
below it, the first no. of that § under the same title. "I. OPEN SETS,
NEIGHBOURHOODS, CLOSED SETS" on page 23 of the same volume is § 1 with the digit
read as the letter it is shaped like, and page 113 has "II. CONNECTEDNESS",
which is § 11 read that way twice.

manifests/toc/ is the authority, as it is for fix heading. It gives every
chapter and every § with the page it opens on, and the number and the title both
have to agree with it. That is what tells a § from its own first no. on page 103,
and it is what keeps this off page 16 of the same volume, where "I. THEORY OF
SETS" and "II. ALGEBRA" list the Books of the Éléments and no § opens at all.

The page keeps its own words. The number is written as the contents gives it,
the title as the page gives it, and the section sign is kept where the page has
one, since Algebra I to III prints it and Topology I to IV does not and the
assembler reads either. A chapter title broken across two lines is joined with a
space, because the break is the width of the measure: page 22 sets "Description"
and "of Formal Mathematics" and the contents calls that chapter DESCRIPTION OF
FORMAL MATHEMATICS. A line the join does not need is left where it stands, so
"(Elementary Theory)" under the title of chapter III of Topology I to IV stays a
line of the page.

An opening the reading dropped altogether is not put back from the page, because
there is nothing there for the contents to agree with and writing the heading in
would mean putting a line on a page that no reading of it ever produced. Twenty
six openings in this corpus are in that state.

-text-layer asks a second reading about those. Every one of these volumes was
scanned by its publisher and most of the PDFs carry the text layer that scan
left, which is a reading of the same paper by a different hand. It is a far
worse reading than the one that made the corpus: page 213 of Algebre I a III has
"5 2. MODULES D'APPLICATIONS LINGAIRES." where the book prints "§ 2. MODULES
D'APPLICATIONS LINÉAIRES.", with the section sign read as a five and an accented
E read as a G. So its words are not words to write down. What it can settle is
whether the heading is on the paper at all, and that is all it is asked.

The number is the whole of the test, as it is everywhere else in this command. A
running head carries the title and no number, which is what keeps this off the
head of page 177 of Topologie generale V a X and on to the real § 6 two lines
below it. The first no. of a § carries the number 1. A wrong match needs a line
numbered as the § is numbered and titled as the § is titled, on the one page the
contents opens that § on.

What gets written is the number and the title the contents gives, set in the
sign and the case the volume sets its other § headings in, which is counted off
the volume rather than guessed: Algebra I to III sets 31 of them with a sign and
in capitals, Groupes et algebres de Lie IV a VI sets 11 with a sign and none in
capitals. A volume with no § heading anywhere in it is left alone and said so,
since there is nothing there to be consistent with. The page is flagged, because
no reading of that page produced that line and somebody going back to the page
image should know it before trusting the wording.

11 of the 26 come back this way. The other 15 are a heading the text layer lost
as well, a page with no text layer at all, or a contents entry pointing at the
wrong page, and those still need the page image read again.

The § mark inside a block of gathered exercises is put back too, and it is put
back on different authority. A chapter that gathers its exercises at the end of
itself writes its Exercises heading once and then marks off each § with the sign
and the number on a line of their own. That line is the least ink on the page and
the reading drops it: fourteen of them are gone here, and unlike the openings the
text layer has only one of them back, since a scan that loses a mark on the paper
loses it again on the second reading.

What stands in for the witness is the block. The contents says which page the §
opens its exercises on, the chapter marks every other § it gathers, and the mark
for this one is on no page of the block. A mark written at the top of that page
contradicts nothing any page says and stands where the printing's own numbering
puts it. The form is copied off the chapter's remaining marks rather than chosen,
since Groupes et algebres de Lie IV a VI sets a stop after the number in chapters
V and VI and none in chapter IV. Where the mark turns up on some other page of
the block the contents is wrong about the page, and that is named and left alone
rather than repaired by writing a second mark.

Those pages carry a flag of their own, worded to say that no second reading is
behind the line. It is a weaker claim than the one the openings make and the page
says so.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID      only this volume, default every volume the contents covers
  -text-layer   ask the text layer about an opening that is not on the page
  -check        say what would change and change nothing
`

const fixFootnoteUsage = `usage: bourbaki fix footnote [flags]

Takes the mark a volume prints beside a footnote off the pages that kept it.

The volumes mark their notes with symbols, restarting on every page: an asterisk
for the first, a dagger for the second, two asterisks for the third. Markdown
numbers its notes itself and prints the number it chose, so a page that keeps
the printed symbol carries two marks for one note, "(*)[^1]" in the body and
"[^1]: (*)" at the foot, and both of them reach the reader.

The symbol is not thrown away. It is the only thing that says which note a
reference belongs to, and the pages this exists for include the ones where the
reading wrote the symbol and no reference at all. So the symbols are read off
the definitions of the page first, and one standing on its own becomes the
reference whose definition carries it. A symbol that two notes of the same file
share, or one whose note is already pointed at from somewhere else, is left
where it is and named: sending a reader to the wrong note is worse than leaving
the printing's mark on the page.

It runs over content/ as well as pages/, which no other repair here does. The
English files are rewritten by the next assemble whatever this does, but a
translation is not: it was made by a model, months of gateway time went into it,
and re-translating fourteen sections over a printer's asterisk is not a trade
anybody should make. A translation is repaired the same way, and its
source_content_sha256 is moved on only when it recorded the English body as it
stood before the repair. One that was already stale stays stale, so L05 still
means what it says.

Run bourbaki assemble afterwards. The pages are the source and the English
sections are made from them, and the two say different things until it runs.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixFenceUsage = `usage: bourbaki fix fence [flags]

Puts back the blank line between a display and the statement head under it.

A display closes on a line holding nothing but two dollars, and what follows it
is a new paragraph. Eight pages of the corpus run the two together: page 314 of
Algebra I to III closes the display defining the mapping pi and then writes
"**Proposition 7.** *The Z-linear mapping (7) is bijective.*" on the very next
line, with no blank line anywhere between them.

Markdown reads that as one block and so does the assembler, which looks for a
head at the start of a block and finds a display. Proposition 7 was therefore
prose, its Corollary 1 was hung on Proposition 6, and chapter II would not
assemble at all: two statements at alg-ii-s6-prop-6-cor-1, one of them the real
corollary of Proposition 6 two pages earlier.

What counts as a head here is assemble.StatesAResult, which is the assembler's
own grammar and not a second opinion about it. The repair exists to hand the
assembler something it can read, so anything else would be repairing pages on a
guess.

Eight pages across six volumes carry this, in both languages, and they are all
the same fault in the same place. Nothing else is touched: a line after a fence
that does not state a result is left where the reading put it.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixSealUsage = `usage: bourbaki fix seal [flags] [path...]

Writes content_sha256 over a section file whose body no longer hashes to it.

Named paths are the only ones sealed. With none, every section is read, which
is the whole corpus unless -lang narrows it, and that is a great deal to reseal
by accident: prefer naming the file the hand correction was in.

The hash is what tells a stale translation from a current one, so nothing may
write it without meaning to, and no command did: assemble writes a section from
its pages and seals it on the way out, and a correction made in content/ by hand
leaves the body one thing and the hash another. S08 then refuses the corpus and
says so, correctly, and there was nothing to run. This is that thing.

It is not a repair of the text and it does not look at the text. It reads what
the file says its body hashes to, hashes the body, and where the two differ it
writes the second. A file already sealed is not rewritten.

manifests/sections.yaml records the same hash a second time and is written with
it, since assemble -check compares the manifest it would write against the
committed one and a section sealed without its row fails that check with no way
to pass it. Only a row the manifest already has is touched.

Sealing an English section restales its translations, which is the point: the
English moved, so the Vietnamese was made from a body that is no longer there.
The translations that recorded the old hash are named, because a hand correction
is usually a comma and a stale translation over a comma is worth knowing about
before the next run spends an hour redoing the section.

Prefer the correction in pages/ where the page is what was misread, since
assemble overwrites the section from the page and the hand correction with it.
Use this where the fault is in the assembly and not in the page.

flags:
  -lang L    only this language, default every language
  -check     say what would change and change nothing
`

func runFix(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, fixUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "section":
		return fixSection(args[1:])
	case "dollars":
		return fixDollars(args[1:])
	case "padding":
		return fixPadding(args[1:])
	case "math":
		return fixMath(args[1:])
	case "stray":
		return fixStray(args[1:])
	case "parens":
		return fixParens(args[1:])
	case "notin":
		return fixNotin(args[1:])
	case "prime":
		return fixPrime(args[1:])
	case "star":
		return fixStar(args[1:])
	case "label":
		return fixLabel(args[1:])
	case "elision":
		return fixElision(args[1:])
	case "smallcaps":
		return fixSmallCaps(args[1:])
	case "dash":
		return fixDash(args[1:])
	case "folio":
		return fixFolio(args[1:])
	case "heading":
		return fixHeading(args[1:])
	case "opening":
		return fixOpening(args[1:])
	case "footnote":
		return fixFootnote(args[1:])
	case "fence":
		return fixFence(args[1:])
	case "seal":
		return fixSeal(args[1:])
	}
	fmt.Fprint(os.Stderr, fixUsage)
	os.Exit(2)
	return nil
}

func fixStray(args []string) error {
	fs := flag.NewFlagSet("fix stray", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixStrayUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, left int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, ok := mathtex.DropStray(f.Body)
		// A page the extractor could not read is not repaired by counting
		// delimiters. See DropStray for the four pages that taught us that.
		if len(f.Meta.Flags) > 0 {
			ok = false
		}
		if !ok {
			// Either there is no unclosed delimiter, which is the usual case,
			// or there is one and this is not the fault it repairs. The second
			// is a page for the repair pass against the printed image, and it
			// is named here so the two are not confused in the summary.
			if _, un := mathtex.Split(f.Body); un != nil {
				left++
				fmt.Fprintf(os.Stderr, "fix stray: left alone, %s:%d is not a stray display delimiter\n",
					rel(root, path), un.Line)
			}
			return nil
		}
		changed++
		if *check {
			fmt.Printf("%s  line %d\n", rel(root, path), strayLine(f.Body))
			return nil
		}
		f.Body = body
		f.Meta.Flags = withFlag(f.Meta.Flags, string(extract.FlagStrayDelimiter))
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "took out"
	if *check {
		verb = "would take out"
	}
	fmt.Printf("fix stray: %d pages read, %s a delimiter on %d of them, %d left alone\n",
		pages, verb, changed, left)
	if changed > 0 && !*check {
		fmt.Println("fix stray: run bourbaki fix math, then bourbaki assemble")
	}
	return nil
}

// strayLine is the line the unclosed delimiter sits on, for the report.
func strayLine(body string) int {
	if _, un := mathtex.Split(body); un != nil {
		return un.Line
	}
	return 0
}

// withFlag adds a flag to a page's flags and keeps them sorted and unique. The
// unbalanced flag stays where it is: a page that balanced only after a
// delimiter was dropped is not a page that was always balanced.
func withFlag(flags []string, f string) []string {
	if slices.Contains(flags, f) {
		return flags
	}
	flags = append(flags, f)
	sort.Strings(flags)
	return flags
}

// eachPage walks the committed pages of one volume, or of every volume, in
// reading order.
func eachPage(root string, books *corpus.BooksManifest, book string, fn func(path string, f *corpus.PageFile) error) error {
	for _, b := range books.Books {
		if book != "" && b.ID != book {
			continue
		}
		names, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, b.ID), "*.md"))
		if err != nil {
			return err
		}
		sort.Strings(names)
		for _, path := range names {
			f, err := corpus.ReadFile[corpus.PageFrontMatter](path)
			if err != nil {
				return err
			}
			if err := fn(path, &f); err != nil {
				return err
			}
		}
	}
	return nil
}

// corpusAndBooks is the two lookups every fix command opens with.
func corpusAndBooks() (string, *corpus.BooksManifest, error) {
	root, err := corpus.Root()
	if err != nil {
		return "", nil, err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return "", nil, err
	}
	return root, books, nil
}

func fixParens(args []string) error {
	fs := flag.NewFlagSet("fix parens", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixParensUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, spans int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := mathtex.Unstraddle(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		spans += n
		if *check {
			word := "spans"
			if n == 1 {
				word = "span"
			}
			fmt.Printf("%s  %d %s\n", rel(root, path), n, word)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "repaired"
	if *check {
		verb = "would repair"
	}
	files, content, followed, err := parensContent(root, *check)
	if err != nil {
		return err
	}

	fmt.Printf("fix parens: %d pages read, %s %d spans in %d of them\n", pages, verb, spans, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix parens: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	if changed > 0 && !*check {
		fmt.Println("fix parens: run bourbaki fix math, then bourbaki assemble")
	}
	return nil
}

// parensContent repairs the same fault in content/, which the pages pass cannot
// reach on its own.
//
// English and French are assembled from their pages, so repairing a page and
// assembling again carries the repair into the section. A translation has no
// page under it. It was written by a model copying the mathematics of a source
// that held the fault, so it holds the fault too, and the only way to it is to
// edit the file. Waiting for the section to be asked for again would work and it
// is not free: the fleet answers a few sections an hour on a good day, and there
// are seven hundred sections with no translation at all waiting behind them.
//
// This is fix footnote's arrangement and it is here for fix footnote's reason. A
// translation whose source is repaired goes stale by its hash, and re-asking it
// over a bracket that moved by a rule neither side had a choice about is not a
// trade anybody should make. So a source is walked first, its body hashed before
// and after, and a translation that recorded the first is moved on to the second.
// Only that translation: one that was already stale stays stale, so L05 still
// means what it says.
//
// A source here is a file that was not translated from anything, which is the
// English and the French alike, and it is not a test on the language. content/en-mt
// is English and it is a translation of the French, and moving it on with the
// French it was made from is the whole point of having the rule at all.
func parensContent(root string, check bool) (files, changed, followed int, err error) {
	return repairContent(root, check, "spans", mathtex.Unstraddle)
}

// repairContent is the walk parens and notin share. A repair of the mathematics
// alone is the same work in every language, so it takes the repair as an
// argument and the two commands differ only in what they call the thing they
// counted.
func repairContent(root string, check bool, unit string, repair func(string) (string, int)) (files, changed, followed int, err error) {
	// The source body each translation was made from, before and after, keyed by
	// the corpus-relative path the translation names.
	moved := map[string][2]string{}
	record := func(path, before, after string) {
		moved[filepath.ToSlash(rel(root, path))] = [2]string{
			corpus.ContentSHA256(before), corpus.ContentSHA256(after)}
	}
	follow := func(from, recorded string) (string, bool) {
		pair, ok := moved[from]
		if !ok || pair[0] != recorded || pair[0] == pair[1] {
			return recorded, false
		}
		return pair[1], true
	}

	var sources bool
	section := func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
		if (f.Meta.TranslatedFrom == "") != sources {
			return nil
		}
		files++
		body, n := repair(f.Body)
		if sources {
			record(path, f.Body, body)
		} else if now, ok := follow(f.Meta.TranslatedFrom, f.Meta.SourceSHA256); ok {
			followed++
			if !check {
				f.Meta.SourceSHA256 = now
			}
			n++
		}
		if n == 0 {
			return nil
		}
		changed++
		if check {
			fmt.Printf("%s  %d %s\n", rel(root, path), n, unit)
			return nil
		}
		f.Body = body
		return f.Write(path) // Write hashes the body again, so the seal follows
	}
	exercise := func(path string, f *corpus.File[corpus.ExerciseFrontMatter]) error {
		if (f.Meta.TranslatedFrom == "") != sources {
			return nil
		}
		files++
		body, n := repair(f.Body)
		if sources {
			record(path, f.Body, body)
		} else if now, ok := follow(f.Meta.TranslatedFrom, f.Meta.SourceSHA256); ok {
			followed++
			if !check {
				f.Meta.SourceSHA256 = now
			}
			n++
		}
		if n == 0 {
			return nil
		}
		changed++
		if check {
			fmt.Printf("%s  %d %s\n", rel(root, path), n, unit)
			return nil
		}
		f.Body = body
		return f.Write(path)
	}
	// The sources first and on their own, because a translation cannot be moved
	// on until the body it was made from has been repaired and hashed twice.
	for _, sources = range []bool{true, false} {
		if err := eachSection(root, "", section); err != nil {
			return files, changed, followed, err
		}
		if err := eachExercise(root, "", exercise); err != nil {
			return files, changed, followed, err
		}
	}
	return files, changed, followed, nil
}

func fixFolio(args []string) error {
	fs := flag.NewFlagSet("fix folio", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixFolioUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	fill := fs.Bool("fill", false, "take the number from the page map when the body has none")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, labelled, disagreed, missing, filled int
	for _, b := range books.Books {
		if *book != "" && b.ID != *book {
			continue
		}
		// A head-number volume is here for -fill and for nothing else. It prints
		// the number in the running head, so the head is where the number is cut
		// from and the body it leaves has no number at the foot to take: the loop
		// below finds none, counts the page as printing none, and fills it from
		// the map like any other. Functions of a Real Variable is the one volume
		// of that kind and none of its 354 pages carried a number at all, so its
		// twenty sections were printed on no page of the book and 701 references
		// that give a page of it resolved to nothing.
		switch pagemap.Grammar(b.Grammar) {
		case pagemap.FootNumber:
		case pagemap.HeadNumber:
			if !*fill {
				continue
			}
		default:
			continue
		}
		pm, err := pagemap.Load(root, b.ID)
		if err != nil {
			return fmt.Errorf("%s: %w: run bourbaki pagemap build -book %s first", b.ID, err, b.ID)
		}
		// A label is only built for a volume that numbers its pages inside the
		// chapter, because that is what a label says. Both foot-number volumes
		// in the corpus number straight through the book, so in practice this
		// writes the number and no label, and says so in the summary.
		letter := corpus.BookLetter(b.Book)
		if pagemap.Pagination(b.Pagination) != pagemap.PerChapter {
			letter = ""
		}
		err = eachPage(root, books, b.ID, func(path string, f *corpus.PageFile) error {
			pages++
			body, folio := corpus.CutFolio(f.Body)
			if folio == 0 {
				if f.Meta.Folio != 0 {
					// Already repaired by an earlier run, which is why the
					// body has no number left to take.
					return nil
				}
				// Most of the rest are a page the volume prints no number on:
				// the opener of a chapter, a plate, the blank facing one. They
				// are not worth a line each, only a count.
				missing++
				e, ok := pm.Lookup(f.Meta.PDFPage)
				if !*fill || !ok || e.Page == 0 || !e.Confidence.Printed() {
					return nil
				}
				filled++
				label := folioLabel(f.Meta.PageLabel, letter, e.Chapter, e.Page)
				if label != f.Meta.PageLabel {
					labelled++
				}
				if *check {
					fmt.Printf("%s  %d  %s  from the page map\n", rel(root, path), e.Page, label)
					return nil
				}
				f.Meta.Folio, f.Meta.PageLabel = e.Page, label
				f.Meta.Flags = withFlag(f.Meta.Flags, folioFromMap)
				return f.Write(path)
			}
			e, ok := pm.Lookup(f.Meta.PDFPage)
			if !ok || e.Page != folio {
				want := 0
				if ok {
					want = e.Page
				}
				disagreed++
				fmt.Fprintf(os.Stderr, "fix folio: left alone, %s prints %d and the page map says %d\n",
					rel(root, path), folio, want)
				return nil
			}
			label := folioLabel(f.Meta.PageLabel, letter, e.Chapter, folio)
			changed++
			if label != f.Meta.PageLabel {
				labelled++
			}
			if *check {
				fmt.Printf("%s  %d  %s\n", rel(root, path), folio, label)
				return nil
			}
			f.Body, f.Meta.Folio, f.Meta.PageLabel = body, folio, label
			f.Meta.Lines = len(strings.Split(strings.TrimSpace(body), "\n"))
			return f.Write(path)
		})
		if err != nil {
			return err
		}
	}

	verb := "took"
	if *check {
		verb = "would take"
	}
	fmt.Printf("fix folio: %d pages read, %s the number off %d of them, %d print none, %d left alone\n",
		pages, verb, changed, missing, disagreed)
	if filled > 0 {
		verb := "took"
		if *check {
			verb = "would take"
		}
		fmt.Printf("fix folio: of those %d, %s the number of %d from the page map\n", missing, verb, filled)
	}
	if labelled > 0 {
		fmt.Printf("fix folio: %d pages got a page label\n", labelled)
	}
	if changed+filled > 0 && !*check {
		fmt.Println("fix folio: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

// folioLabel keeps the label a page already carries, and builds one for a
// volume that numbers inside the chapter and has none.
func folioLabel(has, letter, chapter string, folio int) string {
	if has != "" || letter == "" || chapter == "" {
		return has
	}
	return fmt.Sprintf("%s %s.%d", letter, chapter, folio)
}

// folioFromMap is what a filled page is flagged with, because a number that
// came from somewhere other than the page in front of the reader is worth
// saying so on the page.
//
// Only a number the page map read off the page is filled in, never one it
// worked out from the pages around it. The two look the same in the map and are
// not the same claim at all: the first says the volume prints this number and
// the reading dropped it, which is a repair, and the second says nothing on the
// page carries a number, which makes writing one an invention. A reader holding
// the printed book would go looking for it at the foot and find blank paper.
const folioFromMap = "folio from the page map, printed on the page and dropped by this reading"

func fixHeading(args []string) error {
	fs := flag.NewFlagSet("fix heading", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixHeadingUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}
	man, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}

	var pages, changed, unknown, restored int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		bt, ok := man.Get(f.Meta.Book)
		if !ok {
			// A volume whose contents has not been read yet. There is nothing
			// to look a heading up in, which is not a fault of the page.
			return nil
		}
		lines := strings.Split(f.Body, "\n")
		moved := false
		for i, line := range lines {
			h, ok := toc.ParseHeading(line)
			lost := false
			if !ok {
				// A line the reading took for a paragraph. It is put to the
				// contents the same way a heading is, and only the contents
				// can make it one. See toc.LostHeadingRE for why the shape of
				// the line settles nothing on its own.
				h, ok = toc.ParseLostHeading(line)
				lost = ok
			}
			if !ok {
				continue
			}
			level := toc.Level(*bt, f.Meta.PDFPage, h.Number, h.Title)
			switch {
			case level == 0 && lost:
				// An ordinary numbered paragraph, which the corpus has
				// thousands of. It is not counted with the headings below,
				// because saying the contents does not have it would be
				// claiming it is a heading, and nothing here says it is.
			case level == 0:
				// The contents does not give this heading on this page. The
				// front pages of a volume are mostly this: the contents itself
				// is set as a list of numbered titles and reads as a page full
				// of headings. So it is counted and not printed one by one.
				unknown++
			case level != h.Level:
				lines[i] = h.Write(level)
				moved = true
				if lost {
					restored++
				}
				if *check {
					fmt.Printf("%s:%d  %s\n", rel(root, path), i+1, lines[i])
				}
			}
		}
		if !moved {
			return nil
		}
		changed++
		if *check {
			return nil
		}
		f.Body = strings.Join(lines, "\n")
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "moved a heading on"
	if *check {
		verb = "would move a heading on"
	}
	heading := "headings"
	if unknown == 1 {
		heading = "heading"
	}
	fmt.Printf("fix heading: %d pages read, %s %d of them, %d %s the contents does not have\n",
		pages, verb, changed, unknown, heading)
	if restored > 0 {
		fmt.Printf("fix heading: %d of those had no level at all and were read as paragraphs\n", restored)
	}
	if changed > 0 && !*check {
		fmt.Println("fix heading: run bourbaki assemble")
	}
	return nil
}

// fixOpening walks the contents rather than the pages, which every other
// repair here does the other way round.
//
// The reason is that the fault is a line that is not there, or is there in a
// shape nothing marks as a heading, and neither can be found by reading a page
// and asking what is wrong with it. The contents knows where a chapter and a §
// begin, so the page is opened because the contents sends the repair to it, and
// a volume whose contents has not been read yet has no openings to put back.
// added is the first line a repair put into a body that was not in it before,
// which is the line worth showing under -check. It is written this way rather
// than having the repair hand the heading back because the repairs put their
// heading in three different places and a caller that wants to see it should
// not have to know which.
func added(before, after []string) string {
	for i, line := range after {
		if i >= len(before) || before[i] != line {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// sectionStyle is how a volume sets a § heading: whether it prints the section
// sign, and whether it sets the title in capitals. Both vary by volume and
// neither varies inside one.
//
// It is here because a heading put back from the contents has no page line to
// copy either from. The corpus was counted for it: Algebra I to III sets 31 of
// them, all with a sign and all in capitals, Algebre I a III 26 the same way,
// Groupes et algebres de Lie IV a VI 11 with a sign and none in capitals, and
// Integration IX 6 the same. Topologie generale V a X is the one volume that is
// not unanimous, 13 in capitals against 4 not, and the majority carries it.
type sectionStyle struct {
	sign  bool
	upper bool
	// known says the volume has a § heading somewhere in it to learn from. A
	// volume that has none tells us nothing and gets no heading written.
	known bool
}

// heading is the line the assembler reads, with the number from the contents.
func (s sectionStyle) heading(number int, title string) string {
	if s.upper {
		title = upperOutsideMath(title)
	}
	sign := ""
	if s.sign {
		sign = "§ "
	}
	return "## " + sign + strconv.Itoa(number) + ". " + title
}

// upperOutsideMath capitalises the prose of a title and leaves the mathematics
// alone. § 3 of chapter VII of Topologie generale V a X is titled "Sommes
// infinies dans les groupes $ \mathbf{R}^n $", and upper casing the whole of
// that gives \MATHBF, which is not a command.
func upperOutsideMath(title string) string {
	parts := strings.Split(title, "$")
	for i := 0; i < len(parts); i += 2 {
		parts[i] = strings.ToUpper(parts[i])
	}
	return strings.Join(parts, "$")
}

// setSection is a § heading as this corpus writes one.
var setSection = regexp.MustCompile(`^## (§ ?)?[0-9]{1,3}\. +(.+?)\s*$`)

func sectionStyleOf(root, book string) sectionStyle {
	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, book), "*.md"))
	if err != nil {
		return sectionStyle{}
	}
	var sign, plain, upper, lower int
	for _, path := range paths {
		f, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(f.Body, "\n") {
			m := setSection.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if m[1] != "" {
				sign++
			} else {
				plain++
			}
			if capitals(m[2]) {
				upper++
			} else {
				lower++
			}
		}
	}
	if sign+plain == 0 {
		return sectionStyle{}
	}
	return sectionStyle{sign: sign >= plain, upper: upper >= lower, known: true}
}

// capitals says a title is set in capitals. The mathematics in it is not
// counted, since a title that is half a formula carries lower case letters in
// the formula whatever the press did with the words.
func capitals(title string) bool {
	parts := strings.Split(title, "$")
	letters := false
	for i := 0; i < len(parts); i += 2 {
		for _, r := range parts[i] {
			if unicode.IsLower(r) {
				return false
			}
			if unicode.IsLetter(r) {
				letters = true
			}
		}
	}
	return letters
}

// openAt puts a heading at the top of a page, under whatever blank lines the
// body opens with.
func openAt(lines []string, head string) []string {
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return append(lines[:i:i], append([]string{head, ""}, lines[i:]...)...)
}

// headingFromContents is what a page written this way is flagged with, for the
// same reason folioFromMap exists: a line that came from somewhere other than
// the reading of the page in front of you is worth saying so on the page. The
// text layer witnessed that the heading is printed there, and the words are the
// book's own from its contents pages, but no reading of this page produced this
// line and a person going back to the page image should know that before
// trusting the wording.
const headingFromContents = "heading from the contents, printed on the page and dropped by this reading, witnessed by the text layer"

// markFromContents is what a page gets when the § mark inside a block of
// gathered exercises was put back with no second reading behind it. The flag is
// separate from headingFromContents and worded differently on purpose: that one
// can say the text layer saw the ink and this one cannot, and the difference is
// the whole of what a person going back to the page image needs to know.
const markFromContents = "§ mark from the contents, placed where the chapter's other marks put it, with no second reading of the page behind it"

// headingFromAbsentLeaf is what a page gets when it was given the heading of a §
// the printing sets on the leaf before it, which the scan does not have. It is
// worded to say two things a person going back to the page image needs, because
// both of them are unusual: the words are the book's own from its contents, and
// the line is on this page rather than on the page the press printed it on,
// because the page the press printed it on is not in the file at all. Reading
// this page again will not produce the line and is not what the flag is asking
// for.
const headingFromAbsentLeaf = "heading from the contents, printed on the leaf before this page, which the scan does not have"

func fixOpening(args []string) error {
	fs := flag.NewFlagSet("fix opening", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixOpeningUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	layer := fs.Bool("text-layer", false, "ask the text layer about an opening that is not on the page")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}
	man, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}

	// The text layer of a volume and the way that volume sets a § heading, both
	// read once and both read only when a heading turns out to be lost. The
	// first is a pdftotext run over the whole document and the second is a walk
	// of every page of it, and most volumes need neither.
	text := map[string][]string{}
	style := map[string]sectionStyle{}
	// The style of the volume's § headings is asked for by two repairs and read
	// once. It used to be read as a side effect of reading the text layer,
	// which was fine while the layer was the only thing that wrote a heading.
	// A § headed on a leaf the scan does not have is written without the layer,
	// since the layer of a page that is not in the file does not exist either.
	styled := map[string]bool{}
	styleOf := func(b corpus.Book) sectionStyle {
		if !styled[b.ID] {
			styled[b.ID] = true
			style[b.ID] = sectionStyleOf(root, b.ID)
		}
		return style[b.ID]
	}
	layerPage := func(b corpus.Book, pdfPage int) (string, error) {
		if !*layer || b.TextLayer == "none" {
			return "", nil
		}
		pages, ok := text[b.ID]
		if !ok {
			var err error
			if pages, err = volumeText(context.Background(), root, &b); err != nil {
				return "", err
			}
			text[b.ID] = pages
		}
		if pdfPage < 1 || pdfPage > len(pages) {
			return "", nil
		}
		return pages[pdfPage-1], nil
	}
	// styledHeading is the § heading the contents gives, set the way this
	// volume sets its own. It comes back empty where the volume has no §
	// heading anywhere in it for this to be set like: a heading written in one
	// of the two styles at random would be a guess about the printing on a page
	// nobody has looked at, and the guess would go on the page unmarked.
	styledHeading := func(b corpus.Book, number int, title string) string {
		st := styleOf(b)
		if !st.known {
			return ""
		}
		head := st.heading(number, title)
		if b.Lang == "fr" {
			// The contents manifest sets the apostrophe of an elision straight
			// in 201 of its titles and the French pages set it typographic, so
			// a title carried over as it stands puts a fault on the page that
			// fix elision would then have to come back for. It is the same
			// rule and the same call, made before the line is written rather
			// than after.
			head, _, _ = typography.Apostrophes(head)
		}
		return head
	}
	witness := func(b corpus.Book, pdfPage, number int, title string) (string, string, error) {
		page, err := layerPage(b, pdfPage)
		if err != nil || page == "" {
			return "", "", err
		}
		words, ok := toc.WitnessSection(page, number, title)
		if !ok {
			return "", "", nil
		}
		return words, styledHeading(b, number, title), nil
	}

	var chapters, sections, numbers, appendices, notes, marks, blanks, unread, lost, differ, told, absent int
	for _, b := range books.Books {
		if *book != "" && b.ID != *book {
			continue
		}
		bt, ok := man.Get(b.ID)
		if !ok {
			continue
		}
		// The page map is what says whether a heading nowhere in the file is
		// printed on a leaf the file does not have. A volume with no map yet
		// gets nil, which answers no to that and leaves every repair as it was.
		pm, _ := pagemap.Load(root, b.ID)
		// Every page of a chapter start or a § start is opened at most once
		// and written at most once, since a § and the chapter it opens share
		// a page in most chapters and the two repairs would otherwise write
		// over each other.
		edits := map[int][]string{}
		// A page whose heading was filed as its running head has both put
		// right, so the running head the page really prints is kept here
		// beside the body it belongs to.
		heads := map[int]string{}
		// A page whose heading came from the contents rather than from any
		// reading of the page, which is flagged when it is written.
		witnessed := map[int]bool{}
		// A page whose § mark came from the contents with no second reading
		// behind it, which is flagged differently and says so.
		inferred := map[int]bool{}
		// A page that was given the heading of a § printed on the leaf before
		// it, which the file does not have. Flagged differently again.
		fromLeaf := map[int]bool{}
		read := func(pdfPage int) ([]string, bool) {
			if lines, ok := edits[pdfPage]; ok {
				return lines, true
			}
			f, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, b.ID, pdfPage))
			if err != nil {
				unread++
				return nil, false
			}
			return strings.Split(f.Body, "\n"), true
		}
		for _, ch := range bt.Chapters {
			// A chapter the printing does not have has no opening to put
			// back. See corpus.Chapter.Nominal.
			if ch.PDFPage > 0 && !ch.Nominal {
				lines, ok := read(ch.PDFPage)
				switch {
				case !ok:
				case slices.ContainsFunc(lines, func(l string) bool {
					return strings.HasPrefix(l, "## "+toc.ChapterWord(b.Lang))
				}):
				default:
					out, done := toc.ChapterOpening(lines, b.Lang, ch.Numeral, ch.Title)
					if !done {
						lost++
						fmt.Printf("%s chapter %s: pdf page %d has no line the contents can read as its title\n",
							b.ID, ch.Numeral, ch.PDFPage)
						break
					}
					edits[ch.PDFPage] = out
					chapters++
					if *check {
						fmt.Printf("%s/%04d.md  %s\n", b.ID, ch.PDFPage, out[0])
					}
				}
			}
			// The historical note at the end of a chapter is headed by two
			// words and nothing else, which is the shortest head in the book
			// and the one the reading is likeliest to take for a line of
			// display type it should skip. Nine of them are gone in this
			// corpus and each one stops its volume dead, since assemble looks
			// for the line and there is no other way in to the note.
			if n := ch.Historical; n != nil && n.PDFPage > 0 {
				if err := func() error {
					head, hok := assemble.HistoricalNoteHead(b.Lang)
					if !hok {
						return nil
					}
					lines, ok := read(n.PDFPage)
					if !ok {
						unread++
						return nil
					}
					if slices.Contains(lines, head) {
						return nil
					}
					if at, rest := toc.HistoricalNote(lines); at >= 0 {
						// The words are on the page and only the level was
						// lost, which is the same fault the § and the no.
						// have and takes the same repair. A parenthetical
						// set on the same line stays on it, under the head,
						// because it is a line of the note in the page's own
						// words and there is nowhere else for it to go.
						words := strings.TrimSpace(lines[at])
						lines = slices.Clone(lines)
						lines[at] = head
						if rest != "" {
							lines = slices.Insert(lines, at+1, "", rest)
						}
						edits[n.PDFPage] = lines
						notes++
						fmt.Printf("%s/%04d.md  %s   the page has %q\n", b.ID, n.PDFPage, head, words)
						return nil
					}
					// The head is asked before the text layer because it is
					// the better reading of the two. It is this corpus's own
					// reading of the same page image, filed in the wrong
					// field, where the layer is the publisher's scan of the
					// paper and comes back with the words mangled as often as
					// not.
					if rest, ok := toc.HistoricalNoteFromHead(runningHead(root, b.ID, n.PDFPage)); ok {
						lines = openAt(lines, head)
						if rest != "" {
							// openAt sets the head and a blank line above
							// whatever the body opened on, so the
							// parenthetical goes under the blank and keeps
							// one of its own.
							lines = slices.Insert(lines, slices.Index(lines, head)+2, rest, "")
						}
						edits[n.PDFPage] = lines
						// The words are the whole of what the page prints
						// over a note, so where the reading filed them as
						// the running head there is no head left to keep.
						heads[n.PDFPage] = ""
						notes++
						fmt.Printf("%s/%04d.md  %s   the running head had it\n", b.ID, n.PDFPage, head)
						return nil
					}
					page, err := layerPage(b, n.PDFPage)
					if err != nil {
						return err
					}
					rest, ok := toc.WitnessHistorical(page)
					if !ok {
						lost++
						fmt.Printf("%s chapter %s historical note: pdf page %d has no line the contents can read as its head\n",
							b.ID, ch.Numeral, n.PDFPage)
						return nil
					}
					edits[n.PDFPage] = openAt(lines, head)
					witnessed[n.PDFPage] = true
					notes++
					told++
					fmt.Printf("%s/%04d.md  %s   the text layer has it\n", b.ID, n.PDFPage, head)
					if rest != "" {
						// The layer is too poor a reading to write into a
						// page, so what the head carries after the words is
						// named here and left for somebody to put back.
						fmt.Printf("%s chapter %s historical note: the head on pdf page %d also carries %q, which this does not write\n",
							b.ID, ch.Numeral, n.PDFPage, rest)
					}
					return nil
				}(); err != nil {
					return err
				}
			}
			for _, s := range ch.Sections {
				if s.PDFPage == 0 {
					continue
				}
				// An appendix is opened by its word rather than by a sign and
				// a number, so it takes a repair of its own. Thirty nine of
				// them are in the contents of this corpus and fifteen are not
				// marked on the page, which is what stops the assembler on
				// nine volumes.
				if s.Appendix {
					// The gathered exercises of an appendix are headed by the
					// same word over again, and the reading loses the level
					// there for the same reason it loses it over the opening.
					// Page 449 of Algebra I to III is the exercises of the
					// appendix to chapter II and has the word standing as a
					// plain line in the middle of them.
					//
					// The running head is read here only when the page after
					// carries something else. Every page of an appendix
					// carries the word as its running head, so on its own the
					// front matter of an exercises page says the page is
					// inside the appendix and not that it opens anything. What
					// settles it is the next page of the same run: page 274 of
					// General Topology V to X is headed APPENDIX and page 275
					// is headed EXERCISES, so the word on 274 is not the head
					// of that run and the reading took it off the page.
					if x := s.Exercises; x != nil && x.PDFPage > 0 {
						run := runningHead(root, b.ID, x.PDFPage)
						if strings.EqualFold(strings.TrimSpace(run), strings.TrimSpace(runningHead(root, b.ID, x.PDFPage+1))) {
							run = ""
						}
						if lines, ok := read(x.PDFPage); ok && !toc.Appendix(lines) {
							if out, drop, done := toc.AppendixOpening(lines, run, "", s.Number); done {
								edits[x.PDFPage] = out
								if drop {
									heads[x.PDFPage] = ""
								}
								appendices++
								if *check {
									fmt.Printf("%s/%04d.md  %s\n", b.ID, x.PDFPage, added(lines, out))
								}
							}
						}
					}
					lines, ok := read(s.PDFPage)
					if !ok || toc.Appendix(lines) {
						continue
					}
					out, drop, done := toc.AppendixOpening(lines, runningHead(root, b.ID, s.PDFPage), s.Title, s.Number)
					if !done {
						lost++
						fmt.Printf("%s chapter %s appendix %d: pdf page %d has no line the contents can read as its heading\n",
							b.ID, ch.Numeral, s.Number, s.PDFPage)
						continue
					}
					edits[s.PDFPage] = out
					// The word is the whole of what the page prints over an
					// appendix, so where the reading filed it as the running
					// head there is no running head left to keep.
					if drop {
						heads[s.PDFPage] = ""
					}
					appendices++
					if *check {
						fmt.Printf("%s/%04d.md  %s\n", b.ID, s.PDFPage, added(lines, out))
					}
					continue
				}
				lines, ok := read(s.PDFPage)
				if !ok || toc.Numbered(lines, 2, s.Number) {
					continue
				}
				from, to, head, done := toc.SectionOpening(lines, s.Number, s.Title)
				if !done {
					// The running head is asked before the contents is, and
					// before the text layer, for the same reason the
					// historical note asks it there. It is a reading of this
					// page image made by this corpus, and the only thing
					// wrong with it is the field it was written into.
					run := runningHead(root, b.ID, s.PDFPage)
					put, ok := toc.SectionOpeningFromHead(run, s.Number, s.Title)
					if !ok && locatorSection(root, b.ID, s.PDFPage) == s.Number {
						// The head carries the title and the locator carries
						// the number, which between them are the heading. See
						// toc.SectionOpeningFromLocatedHead.
						put, ok = toc.SectionOpeningFromLocatedHead(run, s.Number, s.Title)
					}
					if ok {
						edits[s.PDFPage] = openAt(lines, put)
						// The heading is the whole of what the page prints
						// above the §, so there is no running head left to
						// keep.
						heads[s.PDFPage] = ""
						sections++
						fmt.Printf("%s/%04d.md  %s   the running head had it\n", b.ID, s.PDFPage, put)
						continue
					}
					if got, ok := toc.SectionTitle(lines, s.Number); ok {
						// A volume that heads a section one way and lists it
						// another in its own contents is disagreeing with
						// itself, and no rule here can tell which of the two
						// it meant. So it is named and left, unless somebody
						// has already looked at both and written the page
						// down in assemble.differs, in which case the words
						// are settled and only the level is missing. The
						// heading then goes back in the page's own words, put
						// to the page's own title rather than to the
						// contents, which is what the record says to keep.
						if assemble.Differs(b.ID, s.PDFPage) {
							if from, to, head, done := toc.SectionOpening(lines, s.Number, got); done {
								lines = append(lines[:from], append([]string{head}, lines[to+1:]...)...)
								edits[s.PDFPage] = lines
								sections++
								fmt.Printf("%s/%04d.md  %s   the contents calls it %q\n",
									b.ID, s.PDFPage, head, s.Title)
								continue
							}
						}
						differ++
						fmt.Printf("%s chapter %s § %d: pdf page %d calls it %q, the table of contents calls it %q\n",
							b.ID, ch.Numeral, s.Number, s.PDFPage, got, s.Title)
					} else if words, put, err := witness(b, s.PDFPage, s.Number, s.Title); err != nil {
						return err
					} else if put == "" && pm.AbsentBefore(s.PDFPage) &&
						styledHeading(b, s.Number, s.Title) != "" {
						// The § is headed on the leaf before this page and the
						// file does not have that leaf, so no reading of the
						// file will ever produce the heading and asking for the
						// page image again would answer nothing. The page map
						// is the authority for that and the only one consulted:
						// it records the step and the printed page that is
						// missing at it.
						//
						// The heading goes at the top of the first page of the
						// § the file does have. That is one printed page later
						// than the press put it, and the alternative is to hang
						// it on the chapter's own first page, which would sweep
						// the chapter title and the conventions paragraph into
						// the § and leave the chapter with no front matter at
						// all. Being one page late is the smaller error and it
						// is the one the flag describes.
						head = styledHeading(b, s.Number, s.Title)
						edits[s.PDFPage] = openAt(lines, head)
						fromLeaf[s.PDFPage] = true
						sections++
						absent++
						fmt.Printf("%s/%04d.md  %s   headed on a leaf the scan does not have\n",
							b.ID, s.PDFPage, head)
					} else if put == "" {
						lost++
						switch {
						case words != "":
							fmt.Printf("%s chapter %s § %d: pdf page %d prints %q in the text layer, and this volume has no § heading anywhere to set it like\n",
								b.ID, ch.Numeral, s.Number, s.PDFPage, words)
						default:
							fmt.Printf("%s chapter %s § %d: pdf page %d has no line the contents can read as its heading\n",
								b.ID, ch.Numeral, s.Number, s.PDFPage)
						}
					} else {
						head = put
						lines = openAt(lines, head)
						edits[s.PDFPage] = lines
						witnessed[s.PDFPage] = true
						sections++
						told++
						fmt.Printf("%s/%04d.md  %s   text layer: %s\n", b.ID, s.PDFPage, head, words)
					}
					continue
				}
				lines = append(lines[:from], append([]string{head}, lines[to+1:]...)...)
				edits[s.PDFPage] = lines
				sections++
				if *check {
					fmt.Printf("%s/%04d.md  %s\n", b.ID, s.PDFPage, head)
				}
			}
			// A chapter that gathers its exercises at the end of itself marks
			// off each § inside the block with the sign and the number on a
			// line of their own, and that line is the whole of what the
			// assembler has to open the §'s run on. It is two or three
			// characters of display type standing alone in white space, which
			// is the least ink on the page, and the reading drops it the way it
			// drops the head of a historical note.
			//
			// The text layer is next to no help here. Fourteen of these marks
			// are gone in this corpus and the layer carries one of them, since
			// a scan that loses a mark on the page loses it again on the second
			// reading. What stands in for the witness is the block itself: the
			// contents says which page the § opens its exercises on, the
			// chapter marks every other § it gathers, and the mark for this one
			// is on no page of the block. A mark written there contradicts
			// nothing the pages say and is where the printing's own numbering
			// puts it.
			//
			// The form is taken from the chapter rather than chosen. Groupes et
			// algebres de Lie IV a VI sets a stop after the number in chapters
			// V and VI and none in chapter IV, and both are copied here from
			// the marks the chapter still has, so the page ends up carrying the
			// printing's mark and not this program's idea of one.
			if err := func() error {
				head, hok := assemble.ExercisesHead(b.Lang)
				if !hok {
					return nil
				}
				type block struct {
					section corpus.Section
					page    int
				}
				var blocks []block
				for _, s := range ch.Sections {
					if x := s.Exercises; !s.Appendix && x != nil && x.PDFPage > 0 {
						blocks = append(blocks, block{s, x.PDFPage})
					}
				}
				stops, plain := 0, 0
				var missing []block
				for _, bl := range blocks {
					lines, ok := read(bl.page)
					if !ok {
						continue
					}
					// A § whose exercises the printing heads in the ordinary
					// way is not gathered and is no evidence either way.
					if slices.Contains(lines, head) {
						continue
					}
					at := slices.IndexFunc(lines, assemble.SectionMark(bl.section).MatchString)
					if at < 0 {
						missing = append(missing, bl)
						continue
					}
					// The mark is on the page and the first exercise is on the
					// line under it, with no blank line between the two. A
					// heading run into the paragraph below it is not a heading
					// at all once the Markdown is read: the two join into one
					// block, the exercises heading the assembler writes over
					// the block goes into the middle of a paragraph, and the §
					// is refused for carrying no exercises on the page its own
					// contents opens them on. Three volumes stop there, and one
					// blank line is the whole of the repair.
					if at+1 < len(lines) && strings.TrimSpace(lines[at+1]) != "" &&
						assemble.ExerciseOpens(lines[at+1]) {
						put := slices.Insert(slices.Clone(lines), at+1, "")
						edits[bl.page] = put
						lines = put
						blanks++
						fmt.Printf("%s/%04d.md  %s   the first exercise was on the line under it\n",
							b.ID, bl.page, strings.TrimSpace(lines[at]))
					}
					if strings.HasSuffix(strings.TrimSpace(lines[at]), ".") {
						stops++
					} else {
						plain++
					}
				}
				if stops+plain == 0 {
					// The chapter marks nothing, so a page with no mark on it
					// is a page with nothing missing.
					return nil
				}
				for _, bl := range missing {
					// The mark may be on some other page of the block, in which
					// case the contents is wrong about the page and writing a
					// second mark is the wrong repair either way.
					re := assemble.SectionMark(bl.section)
					elsewhere := 0
					for _, other := range blocks {
						if other.page == bl.page {
							continue
						}
						if lines, ok := read(other.page); ok && slices.ContainsFunc(lines, re.MatchString) {
							elsewhere = other.page
						}
					}
					if elsewhere > 0 {
						lost++
						fmt.Printf("%s chapter %s § %d: the contents opens its exercises on pdf page %d and the mark is on pdf page %d\n",
							b.ID, ch.Numeral, bl.section.Number, bl.page, elsewhere)
						continue
					}
					lines, ok := read(bl.page)
					if !ok {
						continue
					}
					mark := "§ " + strconv.Itoa(bl.section.Number)
					if stops > plain {
						mark += "."
					}
					page, err := layerPage(b, bl.page)
					if err != nil {
						return err
					}
					edits[bl.page] = openAt(lines, mark)
					marks++
					why := "the block marks every other §"
					if toc.WitnessMark(page, bl.section.Number) {
						why = "the text layer has it"
						witnessed[bl.page] = true
						told++
					} else {
						inferred[bl.page] = true
					}
					fmt.Printf("%s/%04d.md  %s   %s\n", b.ID, bl.page, mark, why)
				}
				return nil
			}(); err != nil {
				return err
			}
			// The no. come after the § and not with them, because the first
			// no. of a § is on the same page as the § and often under the same
			// words, and the § has to be a heading before the no. is looked
			// for or the repair has two lines it cannot tell apart.
			for _, s := range ch.Sections {
				for _, ss := range s.Subsections {
					if ss.PDFPage == 0 {
						continue
					}
					lines, ok := read(ss.PDFPage)
					if !ok || toc.Numbered(lines, 3, ss.Number) {
						continue
					}
					from, to, head, done := toc.NumberOpening(lines, ss.Number, ss.Title)
					switch {
					case done:
						lines = append(lines[:from], append([]string{head}, lines[to+1:]...)...)
					default:
						// The heading may be in the front matter rather than
						// in the body, filed as the running head of the page.
						// That is still a line the reading produced.
						rh, run, ok := toc.RunningHeadOpening(runningHead(root, b.ID, ss.PDFPage), ss.Number, ss.Title)
						if !ok {
							lost++
							fmt.Printf("%s chapter %s § %d no. %d: pdf page %d has no line the contents can read as its heading\n",
								b.ID, ch.Numeral, s.Number, ss.Number, ss.PDFPage)
							continue
						}
						head = rh
						heads[ss.PDFPage] = run
						i := 0
						for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
							i++
						}
						lines = append(lines[:i], append([]string{head, ""}, lines[i:]...)...)
					}
					edits[ss.PDFPage] = lines
					numbers++
					if *check {
						fmt.Printf("%s/%04d.md  %s\n", b.ID, ss.PDFPage, head)
					}
				}
			}
		}
		if *check {
			continue
		}
		for pdfPage, lines := range edits {
			path := corpus.PagePath(root, b.ID, pdfPage)
			f, err := corpus.ReadFile[corpus.PageFrontMatter](path)
			if err != nil {
				return err
			}
			f.Body = strings.Join(lines, "\n")
			// A chapter opening is one line longer than what it replaced,
			// since the number and the title are set on lines of their own,
			// and lines records how long the page is.
			f.Meta.Lines = len(strings.Split(strings.TrimSpace(f.Body), "\n"))
			if run, ok := heads[pdfPage]; ok {
				f.Meta.RunningHead = run
			}
			if witnessed[pdfPage] {
				f.Meta.Flags = withFlag(f.Meta.Flags, headingFromContents)
			}
			if inferred[pdfPage] {
				f.Meta.Flags = withFlag(f.Meta.Flags, markFromContents)
			}
			if fromLeaf[pdfPage] {
				f.Meta.Flags = withFlag(f.Meta.Flags, headingFromAbsentLeaf)
			}
			if err := f.Write(path); err != nil {
				return err
			}
		}
	}

	verb := "put back"
	if *check {
		verb = "would put back"
	}
	fmt.Printf("fix opening: %s %d chapter openings, %d § openings, %d no. openings, %d appendix openings, %d historical notes and %d § marks in gathered exercises\n",
		verb, chapters, sections, numbers, appendices, notes, marks)
	if blanks > 0 {
		fmt.Printf("fix opening: %s the blank line under %d § marks the first exercise was run into\n", verb, blanks)
	}
	if told > 0 {
		fmt.Printf("fix opening: %d of the openings were not on the page at all and came from the contents, with the text layer as the witness that the page prints them\n", told)
	}
	if absent > 0 {
		fmt.Printf("fix opening: %d of the openings are printed on a leaf the scan does not have and were put at the top of the first page of the § the file carries\n", absent)
	}
	if lost > 0 {
		fmt.Printf("fix opening: %s not on the page at all and need the page image read again\n", openings(lost))
	}
	if differ > 0 {
		fmt.Printf("fix opening: %s on the page under a title the contents does not agree with\n", openings(differ))
	}
	if unread > 0 {
		fmt.Printf("fix opening: %s on a page that has not been read\n", openings(unread))
	}
	if chapters+sections+numbers+appendices+notes+marks > 0 && !*check {
		fmt.Println("fix opening: run bourbaki assemble")
	}
	return nil
}

func fixMath(args []string) error {
	fs := flag.NewFlagSet("fix math", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixMathUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var refused []mathtex.Refusal
	var pages, changed, chars int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n, ref := mathtex.Repair(f.Body)
		for i := range ref {
			ref[i].File = rel(root, path)
		}
		refused = append(refused, ref...)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		chars += n
		if *check {
			fmt.Printf("%s  %d characters\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	for _, r := range refused {
		fmt.Fprintln(os.Stderr, "fix math: left alone, "+r.String())
	}
	verb := "repaired"
	if *check {
		verb = "would repair"
	}
	fmt.Printf("fix math: %d pages read, %s %d characters in %d of them, %d left alone\n",
		pages, verb, chars, changed, len(refused))
	if changed > 0 && !*check {
		fmt.Println("fix math: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

const fixPrimeUsage = `usage: bourbaki fix prime [flags]

Braces a primed base so that the power after it is the only power.

TeX reads a prime as a superscript. It is not a character that happens to sit
high, it is \sp{\prime}, so $E'^*$ asks for two superscripts on one atom and TeX
stops on it with "Double superscript". The corpus writes it that way 593 times
over 199 files, in every language, and each one is a volume that will not build
past that line: twelve volumes were failing on it before this command existed,
and the first error is all a reader of the log ever sees, so there is no telling
how many lay behind them.

The repair is the one every TeX manual gives, which is to brace the base:
${E'}^*$. Nothing about the mathematics changes and nothing about the page
changes. A base already braced does not match, so running this twice does
nothing the second time.

A subscript is allowed between the prime and the power, because the corpus
writes that too. $x'_\beta^{m'(\beta)}$ in SS 7 of Algebre I is the same fault
with the subscript in the middle, and the base to brace is the whole of
x'_\beta.

Only mathematics is touched. An apostrophe in a sentence is an apostrophe, and
the corpus has tens of thousands of those in French, which is why this reads the
math spans and not the line.

It runs over content/ as well as pages/, for fix notin's reason. A translation
has no page under it and would otherwise carry the fault until somebody paid a
model to do the section again.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

func fixPrime(args []string) error {
	fs := flag.NewFlagSet("fix prime", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixPrimeUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, powers int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := mathtex.Prime(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		powers += n
		if *check {
			fmt.Printf("%s  %d powers\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	files, content, followed, err := repairContent(root, *check, "powers", mathtex.Prime)
	if err != nil {
		return err
	}

	verb := "braced"
	if *check {
		verb = "would brace"
	}
	fmt.Printf("fix prime: %d pages read, %s %d powers on %d of them\n", pages, verb, powers, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix prime: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	if changed > 0 && !*check {
		fmt.Println("fix prime: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

func fixNotin(args []string) error {
	fs := flag.NewFlagSet("fix notin", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixNotinUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, signs int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := mathtex.Negation(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		signs += n
		if *check {
			fmt.Printf("%s  %d signs\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	// The whole corpus and not one volume, since a translation names the source
	// it was made from and -book is a page filter with nothing to say about it.
	files, content, followed, err := repairContent(root, *check, "signs", mathtex.Negation)
	if err != nil {
		return err
	}

	verb := "struck"
	if *check {
		verb = "would strike"
	}
	fmt.Printf("fix notin: %d pages read, %s %d signs on %d of them\n", pages, verb, signs, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix notin: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	if changed > 0 && !*check {
		fmt.Println("fix notin: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

func fixDollars(args []string) error {
	fs := flag.NewFlagSet("fix dollars", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixDollarsUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, marks int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := textguard.Dollars(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		marks += n
		if *check {
			fmt.Printf("%s  %d delimiters\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	files, content, followed, err := repairContent(root, *check, "delimiters", textguard.Dollars)
	if err != nil {
		return err
	}

	var solutions, solved int
	err = eachSolution(root, "", func(path string, f *corpus.File[corpus.SolutionFrontMatter]) error {
		solutions++
		body, n := textguard.Dollars(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		solved++
		if *check {
			fmt.Printf("%s  %d delimiters\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "wrote"
	if *check {
		verb = "would write"
	}
	fmt.Printf("fix dollars: %d pages read, %s %d delimiters on %d of them\n", pages, verb, marks, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix dollars: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	fmt.Printf("fix dollars: %d solutions read, %d of them changed\n", solutions, solved)
	if changed > 0 && !*check {
		fmt.Println("fix dollars: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

// fix section runs before fix dollars and for the same reason fix dollars runs
// before fix padding: it takes a dollar out of the prose that no formula opened,
// and every pass after it finds its formulas by counting dollars. A page with an
// escaped dollar on it has every span after that point read one boundary out.
func fixSection(args []string) error {
	fs := flag.NewFlagSet("fix section", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixSectionUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, signs int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := textguard.SectionSign(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		signs += n
		if *check {
			fmt.Printf("%s  %d signs\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	// Over content/ as well, for fix parens' reason. A translation has no page
	// under it, so a reference the model copied through in the wrong spelling
	// would otherwise stay in the Vietnamese until somebody paid for the section
	// again.
	files, content, followed, err := repairContent(root, *check, "signs", textguard.SectionSign)
	if err != nil {
		return err
	}

	var solutions, solved int
	err = eachSolution(root, "", func(path string, f *corpus.File[corpus.SolutionFrontMatter]) error {
		solutions++
		body, n := textguard.SectionSign(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		solved++
		if *check {
			fmt.Printf("%s  %d signs\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "wrote"
	if *check {
		verb = "would write"
	}
	fmt.Printf("fix section: %d pages read, %s %d signs on %d of them\n", pages, verb, signs, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix section: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	fmt.Printf("fix section: %d solutions read, %d of them changed\n", solutions, solved)
	if changed > 0 && !*check {
		fmt.Println("fix section: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

// fix padding is fix dollars over the same three trees with the other half of
// the same question: dollars settles which delimiter a formula is written
// between, and this settles whether it is written tight against it. They are
// kept apart because dollars has to run first. Tighten finds its spans by their
// dollars, so a formula still written \( ... \) is not a span it can see, and
// running this before the delimiters are turned round would leave exactly those
// padded.
func fixPadding(args []string) error {
	fs := flag.NewFlagSet("fix padding", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixPaddingUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, spans int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := textguard.Tighten(f.Body)
		// The running head is the one field of a page's front matter that
		// carries mathematics, and 1784 of them carry a padded span: the head
		// of a § called "THE TOPOLOGY OF $ \mathbf{C} $" is read off the page
		// the same way the body is and pads the same way. It matters more than
		// its share of the corpus, because fix opening reads a heading back out
		// of this field, so a head written one way and a heading written the
		// other stop matching.
		head, m := textguard.Tighten(f.Meta.RunningHead)
		if n+m == 0 || (body == f.Body && head == f.Meta.RunningHead) {
			return nil
		}
		changed++
		spans += n + m
		if *check {
			fmt.Printf("%s  %d spans\n", rel(root, path), n+m)
			return nil
		}
		f.Body, f.Meta.RunningHead = body, head
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	files, content, followed, err := repairContent(root, *check, "spans", textguard.Tighten)
	if err != nil {
		return err
	}

	titled, titles, err := tightenTitles(root, *check)
	if err != nil {
		return err
	}

	var solutions, solved int
	err = eachSolution(root, "", func(path string, f *corpus.File[corpus.SolutionFrontMatter]) error {
		solutions++
		body, n := textguard.Tighten(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		solved++
		if *check {
			fmt.Printf("%s  %d spans\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "closed up"
	if *check {
		verb = "would close up"
	}
	fmt.Printf("fix padding: %d pages read, %s %d spans on %d of them\n", pages, verb, spans, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix padding: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	fmt.Printf("fix padding: %d solutions read, %d of them changed\n", solutions, solved)
	fmt.Printf("fix padding: %s %d titles on %d files\n", verb, titles, titled)
	if changed > 0 && !*check {
		fmt.Println("fix padding: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

// tightenTitles is the same repair over the titles rather than the bodies.
//
// A title is not prose and it is not a body, and it goes in four places: the
// section_title, chapter_title and book_title of a section's front matter, the
// subsections list under them, and the chapter, § and no. titles of
// manifests/toc/. All four are read off the same printed contents page as the
// running head and pad the same way, and all four are printed to a reader, in
// the contents of the site and in the contents of a book.
//
// They have to move together with the headings in the bodies, which is the
// reason this is here rather than in a command of its own. manifests/toc/ is
// the authority fix opening and fix heading work to: an opening is written
// from the title in the manifest and then matched against the heading on the
// page, so a manifest tightened without the pages, or pages tightened without
// the manifest, would leave the two spellings on either side of a comparison
// that has to come out equal.
func tightenTitles(root string, check bool) (files, titles int, err error) {
	at := func(s *string) {
		tight, n := textguard.Tighten(*s)
		if n == 0 {
			return
		}
		titles += n
		if !check {
			*s = tight
		}
	}

	err = eachSection(root, "", func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
		was := titles
		at(&f.Meta.BookTitle)
		at(&f.Meta.ChapterTitle)
		at(&f.Meta.SectionTitle)
		for i := range f.Meta.Subsections {
			at(&f.Meta.Subsections[i].Title)
		}
		if titles == was {
			return nil
		}
		files++
		if check {
			fmt.Printf("%s  %d titles\n", rel(root, path), titles-was)
			return nil
		}
		return f.Write(path)
	})
	if err != nil {
		return files, titles, err
	}

	m, err := corpus.LoadTOC(root)
	if err != nil {
		return files, titles, err
	}
	was := titles
	for i := range m.Books {
		for j := range m.Books[i].Chapters {
			c := &m.Books[i].Chapters[j]
			at(&c.Title)
			for k := range c.Subsections {
				at(&c.Subsections[k].Title)
			}
			for k := range c.Sections {
				s := &c.Sections[k]
				at(&s.Title)
				for l := range s.Subsections {
					at(&s.Subsections[l].Title)
				}
			}
		}
	}
	if titles == was {
		return files, titles, nil
	}
	files++
	if check {
		fmt.Printf("manifests/toc/  %d titles\n", titles-was)
		return files, titles, nil
	}
	return files, titles, m.Save(root)
}

func fixStar(args []string) error {
	fs := flag.NewFlagSet("fix star", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixStarUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, stars int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := textguard.Stars(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		stars += n
		if *check {
			fmt.Printf("%s  %d stars\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	files, content, followed, err := repairContent(root, *check, "stars", textguard.Stars)
	if err != nil {
		return err
	}

	verb := "wrote"
	if *check {
		verb = "would write"
	}
	fmt.Printf("fix star: %d pages read, %s %d stars on %d of them\n", pages, verb, stars, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix star: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	if changed > 0 && !*check {
		fmt.Println("fix star: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

// fixLabel takes the item labels of an exercise back out of the mathematics.
func fixLabel(args []string) error {
	fs := flag.NewFlagSet("fix label", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixLabelUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, labels int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := textguard.Labels(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		labels += n
		if *check {
			fmt.Printf("%s  %d labels\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	files, content, followed, err := repairContent(root, *check, "labels", textguard.Labels)
	if err != nil {
		return err
	}

	verb := "wrote"
	if *check {
		verb = "would write"
	}
	fmt.Printf("fix label: %d pages read, %s %d labels on %d of them\n", pages, verb, labels, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix label: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	if changed > 0 && !*check {
		fmt.Println("fix label: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

// fixElision writes the apostrophe of a French elision the corpus's way.
//
// It walks the French printings and no others. The books manifest says what
// language a volume is in, which is better than reading the -fr off the end of
// an id, and the English volumes are left out on purpose rather than left out
// because the rule happens not to fire on them. See fixElisionUsage.
func fixElision(args []string) error {
	fs := flag.NewFlagSet("fix elision", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixElisionUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, marks, left, primes int
	for _, b := range books.Books {
		if b.Lang != "fr" || (*book != "" && b.ID != *book) {
			continue
		}
		err = eachPage(root, books, b.ID, func(path string, f *corpus.PageFile) error {
			pages++
			body, n, over := typography.Apostrophes(f.Body)
			if over > 0 {
				left++
				primes += over
			}
			if n == 0 || body == f.Body {
				return nil
			}
			changed++
			marks += n
			if *check {
				fmt.Printf("%s  %d\n", rel(root, path), n)
				return nil
			}
			f.Body = body
			return f.Write(path)
		})
		if err != nil {
			return err
		}
	}

	verb := "wrote"
	if *check {
		verb = "would write"
	}
	fmt.Printf("fix elision: %d French pages read, %s %d apostrophes on %d of them\n",
		pages, verb, marks, changed)
	if primes > 0 {
		fmt.Printf("fix elision: %d straight apostrophes on %d pages have no elision in front of them and stay as they are\n",
			primes, left)
	}
	if changed > 0 && !*check {
		fmt.Println("fix elision: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

func fixDash(args []string) error {
	fs := flag.NewFlagSet("fix dash", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixDashUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, marks int
	for _, b := range books.Books {
		if b.Lang != "fr" || (*book != "" && b.ID != *book) {
			continue
		}
		err = eachPage(root, books, b.ID, func(path string, f *corpus.PageFile) error {
			pages++
			body, n := typography.StatementDash(f.Body)
			if n == 0 || body == f.Body {
				return nil
			}
			changed++
			marks += n
			if *check {
				fmt.Printf("%s  %d\n", rel(root, path), n)
				return nil
			}
			f.Body = body
			f.Meta.Flags = withFlag(f.Meta.Flags, string(extract.FlagShortHeadDash))
			return f.Write(path)
		})
		if err != nil {
			return err
		}
	}

	verb := "wrote"
	if *check {
		verb = "would write"
	}
	fmt.Printf("fix dash: %d French pages read, %s %d em dashes on %d of them\n",
		pages, verb, marks, changed)
	if changed > 0 && !*check {
		fmt.Println("fix dash: run bourbaki assemble")
	}
	return nil
}

func fixSmallCaps(args []string) error {
	fs := flag.NewFlagSet("fix smallcaps", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixSmallCapsUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, heads, dashed int
	for _, b := range books.Books {
		if b.Lang != "en" || (*book != "" && b.ID != *book) {
			continue
		}
		err = eachPage(root, books, b.ID, func(path string, f *corpus.PageFile) error {
			pages++
			body, n, over := typography.SmallCaps(f.Body)
			dashed += over
			if n == 0 || body == f.Body {
				return nil
			}
			changed++
			heads += n
			if *check {
				fmt.Printf("%s  %d\n", rel(root, path), n)
				return nil
			}
			f.Body = body
			f.Meta.Flags = withFlag(f.Meta.Flags, string(extract.FlagPlainHead))
			return f.Write(path)
		})
		if err != nil {
			return err
		}
	}

	verb := "wrote"
	if *check {
		verb = "would write"
	}
	fmt.Printf("fix smallcaps: %d English pages read, %s %d heads in capitals on %d of them\n",
		pages, verb, heads, changed)
	if dashed > 0 {
		fmt.Printf("fix smallcaps: %d heads are followed by a dash, where the fault is the lost bold and not the case, and stay as they are\n",
			dashed)
	}
	if changed > 0 && !*check {
		fmt.Println("fix smallcaps: run bourbaki assemble")
	}
	return nil
}

// fixFence puts back the blank line between a closing display and the head
// under it.
//
// The head is decided by assemble.StatesAResult, which is the assembler's own
// grammar. See fixFenceUsage.
func fixFence(args []string) error {
	fs := flag.NewFlagSet("fix fence", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixFenceUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}
	// eachPage hands the callback a path and a page and nothing else, and there
	// is a grammar for each language rather than one for the corpus, so the
	// language has to be carried in from the manifest. It is on the volume and
	// not on the page.
	lang := map[string]string{}
	for _, b := range books.Books {
		lang[b.ID] = b.Lang
	}

	var pages, parted, changed int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		lines := strings.Split(f.Body, "\n")
		out := make([]string, 0, len(lines)+4)
		here := 0
		for i, line := range lines {
			out = append(out, line)
			if strings.TrimSpace(line) != "$$" || i+1 >= len(lines) {
				continue
			}
			next := lines[i+1]
			if strings.TrimSpace(next) == "" {
				continue
			}
			if !assemble.StatesAResult(lang[f.Meta.Book], next) {
				continue
			}
			here++
			fmt.Printf("%s  %s\n", rel(root, path), opening(next))
			out = append(out, "")
		}
		if here == 0 {
			return nil
		}
		parted += here
		changed++
		if *check {
			return nil
		}
		f.Body = strings.Join(out, "\n")
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "parted"
	if *check {
		verb = "would part"
	}
	fmt.Printf("fix fence: %d pages read, %s a head from the display above it on %d of them, %d heads in all\n",
		pages, verb, changed, parted)
	if changed > 0 && !*check {
		fmt.Println("fix fence: run bourbaki assemble")
	}
	return nil
}

// opening is the head as it goes into the report, short enough to read a run of
// them down a terminal and long enough to say which statement it is.
func opening(line string) string {
	line = strings.TrimSpace(line)
	r := []rune(line)
	if len(r) <= 64 {
		return line
	}
	return string(r[:64]) + "..."
}

// fixSeal writes content_sha256 over the body it no longer describes.
//
// The two passes are one walk each and not one walk with a lookup, because the
// translations that go stale are named against the hash the English had before
// this run, and that is not known until the first walk is over.
func fixSeal(args []string) error {
	fs := flag.NewFlagSet("fix seal", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixSealUsage) }
	lang := fs.String("lang", "", "only this language")
	check := fs.Bool("check", false, "change nothing")
	named, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}

	// The paths are what to seal, and until now they were parsed and dropped:
	// the return of parseFlags went to _, -lang was the only thing that narrowed
	// the walk, and a run asking for one Vietnamese section resealed 209 of them
	// along with 4 in en-mt. Every hand edit anyone had in the tree went out
	// under a command that had been asked to touch a single file, and undoing it
	// meant reverting by name against git status. A command that rewrites the
	// hash telling a stale translation from a current one is the last one that
	// should quietly do more than it was asked, so the paths now bind.
	only := map[string]bool{}
	for _, a := range named {
		p, err := filepath.Abs(a)
		if err != nil {
			return err
		}
		if _, err := os.Stat(p); err != nil {
			return err
		}
		only[filepath.Clean(p)] = true
	}
	// A path that names a real file the walk never offers is a mistake worth a
	// word rather than a silent no-op: exercises carry no hash of their own and
	// eachSection skips them, so asking to seal one asks for nothing.
	seen := map[string]bool{}

	// The hash each resealed file used to carry, against the file it was in, so
	// that a translation recording it can be named by what it was made from.
	broke := map[string]string{}
	// The new hash against the corpus-relative path, for the manifest.
	now := map[string]string{}
	var read, sealed int
	err = eachSection(root, *lang, func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
		if len(only) > 0 {
			if !only[filepath.Clean(path)] {
				return nil
			}
			seen[filepath.Clean(path)] = true
		}
		read++
		want := corpus.ContentSHA256(f.Body)
		// Every file, and not only the ones sealed here. The manifest row can be
		// stale on its own: seal a section today and the row is written with it,
		// but a section sealed before this command existed left a row behind that
		// nothing has been through since.
		now[filepath.ToSlash(rel(root, path))] = want
		if f.Meta.ContentSHA256 == want {
			return nil
		}
		sealed++
		fmt.Printf("%s  %s is now %s\n", rel(root, path),
			short(f.Meta.ContentSHA256), short(want))
		if f.Meta.ContentSHA256 != "" {
			broke[f.Meta.ContentSHA256] = rel(root, path)
		}
		if *check {
			return nil
		}
		// Write recomputes the hash from the body, so the field is not set here.
		return f.Write(path)
	})
	if err != nil {
		return err
	}
	var missed []string
	for p := range only {
		if !seen[p] {
			missed = append(missed, rel(root, p))
		}
	}
	if len(missed) > 0 {
		sort.Strings(missed)
		return fmt.Errorf("not a section this command can seal: %s",
			strings.Join(missed, ", "))
	}

	// manifests/sections.yaml records the same hash a second time, and the two
	// have to move together: assemble -check compares the manifest it would
	// write against the committed one, and a section sealed here without the
	// manifest is a corpus that fails that check with no way to pass it. The
	// volumes this command is for are the ones assemble will not run on, so
	// rewriting the manifest from a fresh assembly is not open to us.
	rows, err := sealManifest(root, now, *check)
	if err != nil {
		return err
	}

	var stale int
	if len(broke) > 0 {
		err = eachSection(root, "", func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
			from, ok := broke[f.Meta.SourceSHA256]
			if !ok {
				return nil
			}
			stale++
			fmt.Printf("%s  was made from %s as it stood and is now stale\n",
				rel(root, path), from)
			return nil
		})
		if err != nil {
			return err
		}
	}

	verb := "sealed"
	if *check {
		verb = "would seal"
	}
	fmt.Printf("fix seal: %d sections read, %s %d of them and %d manifest rows, %d translations left stale\n",
		read, verb, sealed, rows, stale)
	return nil
}

// sealManifest writes the new hashes into manifests/sections.yaml and returns
// how many rows moved. A row whose hash already agrees is left as it is, and a
// path the manifest does not know is not added: the manifest is assembly's
// account of what it wrote, and a file assembly never wrote does not belong in
// it. Nothing is written when no row moved, so a corpus that is in order comes
// out of this with an unmodified manifest.
func sealManifest(root string, now map[string]string, check bool) (int, error) {
	if len(now) == 0 {
		return 0, nil
	}
	m, err := corpus.LoadSections(root)
	if err != nil {
		return 0, err
	}
	var rows int
	for i := range m.Books {
		for j := range m.Books[i].Chapters {
			for k := range m.Books[i].Chapters[j].Sections {
				r := &m.Books[i].Chapters[j].Sections[k]
				want, ok := now[filepath.ToSlash(r.Path)]
				if !ok || r.ContentSHA256 == want {
					continue
				}
				rows++
				r.ContentSHA256 = want
			}
		}
	}
	if rows == 0 || check {
		return rows, nil
	}
	return rows, m.Save(root)
}

// short is the head of a hash, which is all the report needs and all S08 prints.
func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	if sha == "" {
		return "(nothing)"
	}
	return sha
}

// eachSection walks the section files of the corpus in path order.
//
// The exercises and the solutions are left out. They are files of another
// schema, they carry no content_sha256, and reading them here would only be a
// way of failing on front matter this command has no business parsing.
func eachSection(root, lang string, fn func(path string, f *corpus.File[corpus.SectionFrontMatter]) error) error {
	dir := filepath.Join(root, "content")
	var paths []string
	err := filepath.WalkDir(dir, func(path string, e iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			// content/solutions is a tree of its own schema, and every
			// exercises directory holds the exercises of one §.
			if e.Name() == "solutions" || e.Name() == "exercises" {
				return iofs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		rest, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if lang != "" {
			l, _, _ := strings.Cut(filepath.ToSlash(rest), "/")
			if l != lang {
				return nil
			}
		}
		paths = append(paths, path)
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, path := range paths {
		f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
		if err != nil {
			return err
		}
		if err := fn(path, &f); err != nil {
			return err
		}
	}
	return nil
}

func fixFootnote(args []string) error {
	fs := flag.NewFlagSet("fix footnote", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixFootnoteUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if err := noArgs(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var left int
	// took says how many marks a file gives up and prints the ones it will not.
	// A mark left alone is the interesting half of the report: it is a place
	// where the reading has to be looked at rather than repaired.
	took := func(path string, moves []footnote.Move) int {
		n := 0
		for _, m := range moves {
			if m.Kind == footnote.KindLeft {
				left++
				fmt.Fprintf(os.Stderr, "fix footnote: left alone, %s:%d prints %s and nothing there says which note it means\n",
					rel(root, path), m.Line, m.Mark)
				continue
			}
			n++
			if *check {
				fmt.Printf("%s  line %d  %s %s\n", rel(root, path), m.Line, m.Mark, m.Kind)
			}
		}
		return n
	}

	var pages, repaired int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, moves := footnote.Normalize(f.Body)
		if took(path, moves) == 0 {
			return nil
		}
		repaired++
		if *check {
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	// The English body each translation was made from, before and after, so
	// that a translation recording the first can be moved on to the second.
	// Keyed by the corpus-relative path the translation names.
	moved := map[string][2]string{}
	recordEnglish := func(path, before, after string) {
		moved[filepath.ToSlash(rel(root, path))] = [2]string{
			corpus.ContentSHA256(before), corpus.ContentSHA256(after)}
	}
	// follow moves a translation's record of its source on, and says whether it
	// did. It is deliberately narrow: only a translation that recorded the
	// English body as it stood before this repair, which is the only case where
	// the two are the same translation of the same words.
	follow := func(from, recorded string) (string, bool) {
		pair, ok := moved[from]
		if !ok || pair[0] != recorded || pair[0] == pair[1] {
			return recorded, false
		}
		return pair[1], true
	}

	// The English is walked first and on its own, because a translation cannot
	// be moved on until the body it was made from has been repaired and hashed
	// twice. english says which of the two passes is running.
	var english bool
	var files, content, followed int
	section := func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
		if (f.Meta.Lang == "en") != english {
			return nil
		}
		files++
		body, moves := footnote.Normalize(f.Body)
		n := took(path, moves)
		if f.Meta.Lang == "en" {
			recordEnglish(path, f.Body, body)
		} else if now, ok := follow(f.Meta.TranslatedFrom, f.Meta.SourceSHA256); ok {
			followed++
			if !*check {
				f.Meta.SourceSHA256 = now
			}
			n++
		}
		if n == 0 {
			return nil
		}
		content++
		if *check {
			return nil
		}
		f.Body = body
		return f.Write(path)
	}
	exercise := func(path string, f *corpus.File[corpus.ExerciseFrontMatter]) error {
		if (f.Meta.Lang == "en") != english {
			return nil
		}
		files++
		body, moves := footnote.Normalize(f.Body)
		n := took(path, moves)
		if f.Meta.Lang == "en" {
			recordEnglish(path, f.Body, body)
		} else if now, ok := follow(f.Meta.TranslatedFrom, f.Meta.SourceSHA256); ok {
			followed++
			if !*check {
				f.Meta.SourceSHA256 = now
			}
			n++
		}
		if n == 0 {
			return nil
		}
		content++
		if *check {
			return nil
		}
		f.Body = body
		return f.Write(path)
	}
	// The English first and on its own, because a translation cannot be moved
	// on until the body it was made from has been repaired and hashed twice.
	for _, english = range []bool{true, false} {
		if err := eachSection(root, "", section); err != nil {
			return err
		}
		if err := eachExercise(root, "", exercise); err != nil {
			return err
		}
	}

	verb, verbed := "took", "moved"
	if *check {
		verb, verbed = "would take", "would move"
	}
	fmt.Printf("fix footnote: %d pages read, %s the printed mark off %d of them, %d left alone\n",
		pages, verb, repaired, left)
	fmt.Printf("fix footnote: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	if repaired > 0 && !*check {
		fmt.Println("fix footnote: run bourbaki assemble")
	}
	return nil
}

// eachSolution walks the solution files of one language, or of every language,
// in path order. It is eachSection for the other tree eachSection skips.
//
// content/solutions is laid out by language and then by volume the way the rest
// of content/ is, and everything under it is a solution, so there is no path
// test to make beyond the language: a directory named exercises under it holds
// solutions to exercises and is not the exercises themselves.
//
// The repairs left this tree alone until fix dollars, and mostly still should. A
// solution is prose somebody's own hand wrote rather than a reading of a printed
// page, so a repair whose whole justification is "the page does not say that"
// has nothing to say about it, and fix star in particular would read its bullet
// lists as Bourbaki's mark. The delimiters are the exception, since the corpus
// writes them one way in every tree and a solution written the other way is
// invisible to the audit in exactly the way a page written that way is.
func eachSolution(root, lang string, fn func(path string, f *corpus.File[corpus.SolutionFrontMatter]) error) error {
	dir := filepath.Join(root, "content", "solutions")
	var paths []string
	err := filepath.WalkDir(dir, func(path string, e iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		if lang != "" {
			if l, _, _ := strings.Cut(filepath.ToSlash(mustRel(dir, path)), "/"); l != lang {
				return nil
			}
		}
		paths = append(paths, path)
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, path := range paths {
		f, err := corpus.ReadFile[corpus.SolutionFrontMatter](path)
		if err != nil {
			return err
		}
		if err := fn(path, &f); err != nil {
			return err
		}
	}
	return nil
}

// eachExercise walks the committed exercise files of one language, or of every
// language, in path order. It is eachSection for the tree eachSection skips.
func eachExercise(root, lang string, fn func(path string, f *corpus.File[corpus.ExerciseFrontMatter]) error) error {
	dir := filepath.Join(root, "content")
	var paths []string
	err := filepath.WalkDir(dir, func(path string, e iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if e.Name() == "solutions" {
				return iofs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		rest := filepath.ToSlash(mustRel(dir, path))
		if !strings.Contains(rest, "/exercises/") {
			return nil
		}
		if lang != "" {
			if l, _, _ := strings.Cut(rest, "/"); l != lang {
				return nil
			}
		}
		paths = append(paths, path)
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, path := range paths {
		f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](path)
		if err != nil {
			return err
		}
		if err := fn(path, &f); err != nil {
			return err
		}
	}
	return nil
}

// mustRel is filepath.Rel where the two paths are known to share a root,
// because the walk produced the second from the first.
func mustRel(base, path string) string {
	rest, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rest
}

// runningHead is what a page file records as the running head of the page, and
// the empty string where the page is not there. fixOpening reads the body of a
// page through read and this is the one thing it wants out of the front matter.
// locatorSection is the § the page itself says it is in, and 0 where the page
// says nothing.
func locatorSection(root, book string, pdfPage int) int {
	f, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, book, pdfPage))
	if err != nil || f.Meta.Locator == nil {
		return 0
	}
	return f.Meta.Locator.Section
}

func runningHead(root, book string, pdfPage int) string {
	f, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, book, pdfPage))
	if err != nil {
		return ""
	}
	return f.Meta.RunningHead
}

// openings counts openings and agrees with itself about the verb, since these
// three lines are read one at a time and "1 openings are" reads as a bug in the
// thing that printed it.
func openings(n int) string {
	if n == 1 {
		return "1 opening is"
	}
	return fmt.Sprintf("%d openings are", n)
}
