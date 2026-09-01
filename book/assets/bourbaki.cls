% bourbaki.cls sets a volume of the Elements the way the printing sets it.
%
% Everything here was measured off a scan rather than guessed. The yellow is
% #FEC746 and it is that on both of the two French Springer volumes that were
% sampled, over 85 per cent of the cover pixels on each. The blue of the lettering
% is about #0077B4, noisier because the cover is a JPEG inside the PDF. The four
% bands of the cover sit at the same fractions of the trim height on two volumes
% of different trim, which is what says they are fractions and not millimetres.
%
% The page is the one exception and it is a deliberate one. It used to come from
% manifests/books.yaml, which took it from whichever PDF the volume was read out
% of, so a built Algebra I was 128 by 199.5 mm because the 1998 reprint on the
% shelf is. Every volume is now set on Springer's monograph page instead, and the
% measurements and the reasoning are written beside \bpaper.
%
% One thing this does not do is reproduce the Springer logo, which is somebody
% else's mark and not part of the mathematics.

\NeedsTeXFormat{LaTeX2e}
\ProvidesClass{bourbaki}[2026/08/26 A volume of the Elements de mathematique]

\LoadClass[10pt,twoside]{book}

% no-math because the mathematics is set by the packages below and by the symbol
% font declarations further down, not by fontspec. Without it fontspec claims the
% math families for itself and those declarations are overwritten.
\RequirePackage[no-math]{fontspec}
% leqno because Bourbaki numbers a formula on the left. 1108 displays in the
% corpus carry \tag and the printing has every one of them flush left, which is
% one option rather than 1108 edits. It has to be given to amsmath here and not
% to the class above: an option handed to \LoadClass is not a global document
% option, so amsmath never sees it and numbers on the right regardless, which is
% what the first attempt did.
\RequirePackage[leqno]{amsmath}
\RequirePackage{amssymb}
\RequirePackage{mathrsfs}
% amssymb and mathrsfs on Computer Modern, which is the mathematics of the face
% the text is set in. There is no third math package here and there used to be:
% newtxmath, the Times mathematics, went out when the text face did, and the
% reason is under \setmainfont.
%
% What the two above give between them is what the reference volume embeds.
% pdffonts on Springer Nature's 2023 Algebra Chapter 8 lists CMEX10 for the big
% operators, LMMathItalic and LMMathSymbols for the letters and the signs, rsfs10
% for the script capitals, and nothing else. pdffonts on this build lists CMEX10,
% CMMI, CMSY, MSAM and MSBM out of amssymb, EUFM for the fraktur and rsfs10 out
% of mathrsfs. LMMathItalic and CMMI are the same design under two names, one cut
% for the OpenType release and one for the original Type 1.
%
% unicode-math with STIX Two Math was tried before either of these and abandoned,
% and the reason is worth keeping because it will be tried again otherwise.
% unicode-math defines every symbol as a \Umathchar primitive. hyperref, when it
% turns a heading into a PDF bookmark string, throws the math shift away and then
% tries to expand what was inside it, and a \Umathchar is not something it can
% expand. The build died on a real heading, SS 4 of chapter II of Algebre, whose
% title carries \otimes, with "Improper alphabetic constant". These symbols are
% ordinary \mathchar definitions and hyperref has digested those for thirty
% years.
%
% The corpus uses 283 distinct control sequences inside its math spans and every
% one of them was probed against this stack. It misses \widecheck, which has a
% fallback below, and nothing else. \overparen and \wideparen used to be pointed
% at newtxmath's \widearc and no longer can be; the corpus turns out to use
% neither, so the two \providecommand lines that did it are gone with the
% package.
\RequirePackage{graphicx}
\RequirePackage{xcolor}
\RequirePackage{eso-pic}
\RequirePackage{geometry}
\RequirePackage{enumitem}
\RequirePackage[hidelinks]{hyperref}

% \widecheck has no package in the bundle and is faked with \check. It is one of
% the four control sequences the probe says neither stack carries.
\providecommand{\widecheck}[1]{\check{#1}}

% The operator names the French printing uses and no TeX engine has heard of.
%
% A reading of TG VIII SS 2 or of FVR VI writes tg for the tangent and cotg for
% the cotangent and ch, sh, th for the hyperbolic three, because that is what is
% printed. A reading of AC VIII SS 2 writes dimgr and htgr, set solid and not
% spaced, for the graded dimension and the graded height, and AC X writes prof
% for the depth and Ass for the associated primes. The historical note to FVR
% quotes Leibniz in his own hand and his integral is omn., for omnia. Every one
% of these is the right reading of the sign in front of it, and every one of
% them stops the typesetter, so the class is where they belong and not the
% pages. A census over pages/ found 150 occurrences across 28 pages of nine
% printings, and none of the nine would have built.
%
% \th is the one worth naming separately. LaTeX defines it already, as the
% thorn of Old English, and only in text; in a formula it is an error rather
% than a wrong glyph. Taking the name costs nothing the corpus uses, since
% there is no thorn anywhere in the Elements and there are hyperbolic tangents
% through the whole of FVR and TG.
\DeclareMathOperator{\tg}{tg}
\DeclareMathOperator{\cotg}{cotg}
\DeclareMathOperator{\ch}{ch}
\DeclareMathOperator{\sh}{sh}
\DeclareMathOperator{\cosec}{cosec}
\DeclareMathOperator{\codim}{codim}
\DeclareMathOperator{\dimgr}{dimgr}
\DeclareMathOperator{\htgr}{htgr}
\DeclareMathOperator{\prof}{prof}
\DeclareMathOperator{\Ass}{Ass}
\DeclareMathOperator{\omn}{omn}
\let\th\relax
\DeclareMathOperator{\th}{th}

% The arc over an inverse image, which E II SS 3 prints as a wide arc with the
% -1 sitting above it. A reading names it \overparen or \wideparen, after the two
% packages that carry it, and neither package can be loaded here: yhmath takes
% over \widehat and mathabx takes over most of the symbols. These used to be
% pointed at newtxmath's \widearc, which is the same sign under a third name, and
% newtxmath is no longer loaded.
%
% A census over the whole corpus finds no \overparen and no \wideparen in any
% math span, so there is nothing to point anywhere. \widetilde is what the
% readings settled on and it is in every stack. If one of the two names comes
% back, \overgroup is not the substitute: it draws a flat bar with the ends
% turned down and the printing has a curve.

% The two interior products of Algebre III SS 11 are printed as corners, and the
% printing spaces them the way it spaces any other binary operator: x is set
% away from the corner and so is u. amssymb declares both corners as closing
% delimiters, which is right for the corner of a diagram and wrong here, and it
% sets the formula as x_|u with the three glyphs touching. The corpus uses these
% two signs for the interior product and for nothing else, in Algebre III SS 11,
% Algebre IX SS 9 and Lie IX SS 6, so the class can say so once for all of them.
\let\bcornerlr\lrcorner
\let\bcornerll\llcorner
\renewcommand{\lrcorner}{\mathbin{\bcornerlr}}
\renewcommand{\llcorner}{\mathbin{\bcornerll}}

% amsmath stops a matrix at ten columns and says so. The corpus has none wider,
% but it has 573 matrices and the one that goes over would stop the build in a
% file nobody is looking at, so the counter goes up rather than the build down.
\setcounter{MaxMatrixCols}{20}

% Latin Modern, which is what Springer sets the Elements in.
%
% This was Latin Modern once, was changed to a Times, and is Latin Modern again.
% The argument for the Times was that a Springer mathematics volume is set in
% one, that the Graduate Texts and the Universitext and the Grundlehren are all
% Times, and that Latin Modern is the face a document gets when nobody chose
% one. Every clause of that is true of Springer in general and none of it is
% true of this series, which is the thing being set.
%
% There is exactly one Bourbaki in pdf/ that is a modern Springer setting rather
% than a photograph of a Hermann page, the Algebra Chapter 8 that Springer Nature
% typeset in 2023, and pdffonts on it reads LMRoman10-Regular, LMRoman10-Italic,
% LMRomanCaps10-Regular, LMRoman6, 7, 8, 9 and 12 for the smaller sizes, and the
% Latin Modern math fonts. The Times faces in that file are on the four pages of
% Springer's own front matter and nowhere in the mathematics. So the question of
% what Springer sets the Elements in has an answer that can be read off a file
% rather than argued from the rest of their list, and the answer is this.
%
% Setting the two side by side on the same page of real Bourbaki, at the measure
% and the leading under \bpaper, the Times is the darker page and the Latin
% Modern is the reference page. The volume runs about six per cent longer in
% Latin Modern, 431 pages against 407 for the English Algebra VIII, because the
% face is wider.
%
% Vietnamese was the condition the Times was kept for once and it turns out not
% to be a condition at all: lmroman10-regular.otf carries the whole of Latin
% Extended Additional. Tiếng Việt, Đại số, Bổ đề, Định lý, Mệnh đề and Hệ quả set
% in it with no missing character, and so do the 37 Vietnamese volumes. The
% Latin Modern that does not is the Type 1 cut from before the OpenType release,
% which is what the earlier note here was remembering.
%
% The small caps are real and they are in a file of their own. \textsc is what
% every statement head in the book uses and lmroman10-regular has no small caps
% feature, so lmromancaps10-regular is named as the SmallCapsFont and there is no
% fontspec warning about faking them.
%
% The fonts are named by file rather than by family. tectonic ships them inside
% its bundle rather than installing them, so \setmainfont{Latin Modern Roman}
% fails on a machine that has no TeX Live and works on one that has, which is
% exactly the kind of difference this build is meant not to have. The file names
% resolve out of the bundle on any machine, and every one below was checked
% before it was written down.
\setmainfont{lmroman10-regular.otf}[
  BoldFont       = lmroman10-bold.otf,
  ItalicFont     = lmroman10-italic.otf,
  BoldItalicFont = lmroman10-bolditalic.otf,
  SmallCapsFont  = lmromancaps10-regular.otf,
]
% The sans and the mono of the same family, so that the three faces of the
% document are three cuts of one design rather than three designs.
%
% Nothing in the book is set in the sans: the running heads are small capitals of
% the text face and the corpus has no sans in it anywhere. It is named so that a
% document that ever asks for one gets Latin Modern's and not a Helvetica beside
% a Latin Modern page. The mono has one use, the four character permanent tag in
% the margin, set at 7 pt in grey.
\setsansfont{lmsans10-regular.otf}[
  BoldFont       = lmsans10-bold.otf,
  ItalicFont     = lmsans10-oblique.otf,
  BoldItalicFont = lmsans10-boldoblique.otf,
]
\setmonofont{lmmono10-regular.otf}[
  BoldFont   = lmmonolt10-bold.otf,
  ItalicFont = lmmono10-italic.otf,
]

% Protrusion, which is the difference between a page that was justified and a
% page that was printed. A comma or a hyphen at the end of a line is nearly all
% white, and a line that stops with its comma inside the measure has a notch in
% the right margin that the eye reads as a ragged edge. microtype hangs those
% characters a little way past the margin so the edge of the text block is the
% edge of the letters, which is what a printing house does with metal and what
% every Springer volume has.
%
% Only protrusion. Font expansion is the other half of what microtype does and
% XeTeX cannot do it, so asking for it gets a warning on every build and nothing
% else. Turning it off here says that was a decision.
\RequirePackage[protrusion=true,expansion=false]{microtype}

% A widow and an orphan are the two faults a reader notices without knowing the
% words for them: one line of a paragraph alone at the top of a page, or the
% first line alone at the bottom. LaTeX's default penalty of 150 is a preference
% and this build overrode it in practice anyway, because \flushbottom and a
% \parskip of zero leave the page maker almost nothing to stretch, so it took
% the widow instead. 10000 makes it a prohibition. \displaywidowpenalty is the
% same rule for a paragraph broken above a display, which this book is full of.
\widowpenalty=10000
\clubpenalty=10000
\displaywidowpenalty=10000

% An array set solid. The rows of an array are set on \baselineskip like any
% other line, and \linespread below squeezes that to ten on eleven, which is
% right for prose and too tight for a diagram whose rows are alternately an
% object and an arrow hanging from it. The arrows touch the letters above them.
% This gives the rows of every array, and so of every matrix and every table,
% about a point and a half more, which is what the printed diagrams have.
\renewcommand{\arraystretch}{1.15}

% A labelled vertical arrow of a commutative diagram. The label is set in a box
% of no width so that the arrow, and not the arrow and its label together, is
% what the column centres on. book/diagram.go is what decides which cells get
% this and why, and it only ever hands one a label with an empty cell beside it
% for the box to hang into.
%
% Script size because \xrightarrow sets the labels of the horizontal arrows at
% script size, and a diagram whose two kinds of label are two different sizes
% looks like two diagrams.
\newcommand{\bvarrowr}[2]{#1\rlap{$\scriptstyle\,#2$}}
\newcommand{\bvarrowl}[2]{\llap{$\scriptstyle#2\,$}#1}

% The running head face, which is the text face with the letters tracked apart.
% The printing letterspaces the small capitals in its heads, which is what a
% French mathematical printing of that period does, and it is why the printed
% head is wider than the same words set solid. Measurement is under \bheadfont
% further down. Nothing else in the book is tracked.
\newfontfamily\bheadface{lmromancaps10-regular.otf}[LetterSpace=2]

% The roman inside a formula. amssymb sets the symbols and the CM math fonts set
% the letters, but the upright alphabet and the digits come from whatever the
% kernel has, and with fontspec in no-math mode that is whatever the last class
% to touch it left there. These lines say it explicitly, so that every digit,
% every parenthesis and every \operatorname{Card} in the book is the same face as
% the sentence around it.
%
% lmr, lms and lmtt are the Type 1 Latin Modern roman, sans and typewriter, which
% are the same three faces named above in the encoding the math machinery wants.
\DeclareSymbolFont{operators}{T1}{lmr}{m}{n}
\SetSymbolFont{operators}{bold}{T1}{lmr}{b}{n}
\DeclareMathAlphabet{\mathrm}{T1}{lmr}{m}{n}
\DeclareMathAlphabet{\mathbf}{T1}{lmr}{b}{n}
\DeclareMathAlphabet{\mathit}{T1}{lmr}{m}{it}
\DeclareMathAlphabet{\mathsf}{T1}{lmss}{m}{n}
\DeclareMathAlphabet{\mathtt}{T1}{lmtt}{m}{n}

% The scripts the text face has nothing for. The corpus holds 239 characters of
% Devanagari, Cyrillic, Armenian, Greek and Arabic, nearly all of them in the
% historical notes quoting a source in its own alphabet, and the rest of them
% OCR noise that a reader still has to be able to see in order to report. A hole
% on the page is the worst possible outcome of either.
%
% FreeSerif because it is in tectonic's bundle and because it covers all five.
% That matters more than it sounds: this used to name Arial Unicode MS and fall
% back to Noto Serif, both of them fonts a particular machine happens to have
% installed, so the same volume built on two machines came out with the Greek
% set on one and a gap on the other. A face out of the bundle is the same face
% everywhere, which is the whole reason the text fonts are named by file too.
%
% It is a Times-metric serif, which is a shade darker and a shade narrower than
% the Latin Modern around it. That is visible if you go looking for it and it is
% the right trade: the alternative in the bundle that matches Latin Modern for
% weight has none of the five scripts, and a Greek word set a little dark is a
% Greek word a reader can read.
%
% book/unicode.go decides what goes inside this, and it wraps whole runs rather
% than single characters so that no word is set in two faces.
\IfFontExistsTF{FreeSerif.otf}{%
  \newfontfamily\bfallbackfont{FreeSerif.otf}[
    BoldFont       = FreeSerifBold.otf,
    ItalicFont     = FreeSerifItalic.otf,
    BoldItalicFont = FreeSerifBoldItalic.otf,
  ]%
  \newcommand{\bscript}[1]{{\bfallbackfont #1}}%
  \newcommand{\bgreek}[1]{{\bfallbackfont #1}}%
}{%
  \newcommand{\bscript}[1]{#1}%
  \newcommand{\bgreek}[1]{#1}%
}
% \bgreek is the same face and a different name on purpose. A Greek word quoted
% as Greek is not the same thing as a character the text face happens to lack:
% the first is what the printing has and the second is something to go and look
% at. Two commands means the audit can say so, and it means the day a Greek face
% that matches Latin Modern for weight turns up in the bundle, the quotations
% move to it and the Devanagari does not.
% A bookmark is a string and not a page, so a face has no meaning in one. Left
% alone hyperref tries to expand \bfallbackfont into the outline and writes a
% warning for every heading that quotes a word in another alphabet. This says
% the wrapper is the identity there, which keeps the word in the bookmark and
% drops only the face. It is the same reason the mathematics is set by ordinary
% \mathchar declarations and not by unicode-math, written up at the top of this
% file: hyperref has to be able to expand whatever it finds in a heading.
\pdfstringdefDisableCommands{\let\bscript\@firstofone\let\bgreek\@firstofone}

% The dangerous bend. Bourbaki sets a road sign in the margin against a passage
% a reader can go badly wrong in, and the corpus holds 35 of them. Almost no text
% font has the character, and Knuth's manual font does, so one font is loaded for
% one glyph.
\font\bmanual=manfnt at 10pt
\providecommand{\dbend}{{\bmanual\char127}}

% The math extension font, the one that holds the big parentheses and the big
% sums, at every size from the one file. A Vietnamese running head that carries
% math asked for cmex8, which is a real font and is not in tectonic's bundle,
% and the build died at the last step with "cannot proceed without a physical
% font". The English volume happened not to have a head with math in it and
% built. This says that any size of the extension font is cmex10 at that size,
% which is what plain LaTeX has always done, and it takes the difference between
% the two languages out of the build.
%
% This line was nearly dropped when newtxmath was loaded, on the grounds that
% newtxmath brings its own extension font and the declaration would never fire.
% With the mathematics back on the Computer Modern fonts it fires again and it is
% the only thing standing between a Vietnamese head that carries math and a build
% that stops at the last step.
\DeclareFontShape{OMX}{cmex}{m}{n}{<->cmex10}{}

\definecolor{bourbakiyellow}{HTML}{FEC746}
\definecolor{bourbakiblue}{HTML}{0077B4}

% ---------------------------------------------------------------------------
% The volume's own facts, which the generated document sets before it begins.
% ---------------------------------------------------------------------------

\newcommand{\bvolumetitle}{}
\newcommand{\bvolumespan}{}
\newcommand{\bvolumeseries}{ÉLÉMENTS DE MATHÉMATIQUE}
\newcommand{\bvolumeauthor}{N. BOURBAKI}
\newcommand{\bvolumeedition}{}

\newcommand{\btitle}[1]{\renewcommand{\bvolumetitle}{#1}}
\newcommand{\bspan}[1]{\renewcommand{\bvolumespan}{#1}}
\newcommand{\bseries}[1]{\renewcommand{\bvolumeseries}{#1}}
\newcommand{\bedition}[1]{\renewcommand{\bvolumeedition}{#1}}

% \bpaper is the page, and it is one page for every volume of the shelf rather
% than the trim of whichever printing the manifest happens to name.
%
% The earlier version of this took the trim out of the manifest and set the
% margins as fractions of it, measured off the English Algebra I to III of 1998:
% a 128 by 199.5 mm trim with a 6.5 mm margin on each side, giving a 116 mm
% measure of ten point type led on 10.8 pt. Those numbers are right for that
% book, because that book is a photographic reprint of the Hermann original and
% Hermann set it on cheap paper in 1970 with as little white as it could manage.
% Reproducing it faithfully reproduces a page nobody would set today: eighty
% eight characters to the line with barely a point of lead between them.
%
% The numbers below are measured off the one Bourbaki in the sources that is a
% modern Springer setting rather than a reprint, the Algebra Chapter 8 that
% Springer Nature typeset in 2023. Six body pages of it give the same block
% every time:
%
%   trim         439.37 by 666.14 pt, which is 155 by 235 mm
%   side margins 52.7 pt each, the same on the recto and the verso
%   measure      333.9 pt, 117.8 mm, about 74 characters of ten point roman
%   first line   54 to 59 pt from the head of the page
%   last line    628.6 pt down, so 37.5 pt of foot margin
%   leading      14.35 pt
%
% 155 by 235 mm is Springer's own monograph trim and 117 mm is the measure their
% class sets on it. The leading is more open than that class's ten on twelve,
% and it is what makes the reference page read the way it does, so it is kept.
%
% What this costs is the facsimile. A volume no longer comes out the size its
% printing came out, and the audit line that reports the page count says so.
% That line was never a gate, for the reason written beside it: two volumes out
% of the same corpus already landed at 95 and 74 per cent of their printings
% because the printings are two different books to look at.
%
% The margin note column carries a four character permanent tag at 7 pt in a
% monospaced face, which measures 14.7 pt. The old 6.5 mm margin had no room for
% it and it had to be squeezed in; 18.6 mm has room for it and for a gap wide
% enough to read across. includemp=false because geometry's default is to count
% that column as part of the body, and left at the default it takes 28 pt off
% the measure for a column that is empty on most pages.
%
% The head box is 18 pt and the gap under it is 8.5 pt, and between them they
% are the whole of the running head problem this class had.
%
% It was 12 pt and 10 pt, and the head did not fit in 12 pt. LaTeX sets the
% head in a \vbox to \headheight, and a head line that is taller than that
% overflows it and gets reported. That was 331 of the 363 overfull boxes in a
% build of the French Groupes et algebres de Lie, and every other volume of the
% library had the same thing on nearly every page of it. Between 13 and 13.5 pt
% is enough for a body head in this volume and 15.7 pt is what its contents
% pages want, because the head there is TABLE DES MATIERES with a grave on the
% E; a Vietnamese head with two diacritics stacked on one letter wants more
% again. 18 pt is over all of them with room to spare.
%
% Room to spare costs nothing, which is the part worth knowing. LaTeX puts a
% \vfil above the head line inside that box, so the line hangs from the bottom
% of the box and not the top, and the bottom of the box is \headsep above the
% text block. Where the head sits on the page is therefore \headsep and the
% depth of the line, and \headheight only says how far up the box is allowed to
% go into a margin that has nothing else in it. So the number can be generous
% and the head does not move.
%
% \headsep goes from 10 pt to 8.5 pt to keep the head where the reader has been
% seeing it. While the box was overflowing, the line was pushed down out of the
% bottom of it by about a point and a half, and that is the page every built
% volume up to now has had. 8.5 pt puts it back in the same place, and this
% time it is there because it was asked to be.
%
% The warnings read "Overfull \vbox while \output is active", which sounds like
% the page and is not the page. It was looked for in the text block twice, once
% in \textheight and once in the glue on the paragraph, and both times a change
% that should have made a difference made none. That is the tell, and it is
% worth writing down: if a page of prose is a fraction of a point too tall and
% giving it shrink does not help, the box that is overflowing is not the one
% holding the prose.
\newcommand{\bpaper}{%
  \geometry{paperwidth=155mm,paperheight=235mm,%
    inner=18.6mm,outer=18.6mm,%
    top=12mm,headheight=18pt,headsep=8.5pt,%
    textheight=574.1pt,footskip=8mm,%
    includemp=false,%
    marginparwidth=7mm,marginparsep=2.5mm}%
}

% ---------------------------------------------------------------------------
% The cover.
% ---------------------------------------------------------------------------

% \bcoverline sets one line of the cover at a measured fraction of the trim
% height, centred, and at a width given as a fraction of the trim width. The
% wordmark and the series line are each a fixed 136 mm wide on both of the
% volumes that were measured, whatever the trim, so they are scaled to a width
% rather than set at a size.
\newcommand{\bcoverline}[3]{%
  \AtPageUpperLeft{%
    \put(0,-#1\paperheight){%
      \makebox(0,0)[lt]{\parbox{\paperwidth}{\centering
        \color{bourbakiblue}\resizebox{#2\paperwidth}{!}{#3}}}}}%
}

% \bcoverset sets a line at a size instead of a width, for the title and the
% chapter span, which are set in type on the printed cover rather than drawn.
\newcommand{\bcoverset}[3]{%
  \AtPageUpperLeft{%
    \put(0,-#1\paperheight){%
      \makebox(0,0)[lt]{\parbox{\paperwidth}{\centering
        \color{bourbakiblue}\fontsize{#2}{#2}\selectfont #3}}}}%
}

\newcommand{\bcover}{%
  \thispagestyle{empty}%
  \newgeometry{margin=0pt}%
  \pagecolor{bourbakiyellow}%
  \AddToShipoutPictureBG*{\bcoverline{0.050}{0.78}{\bvolumeauthor}}%
  \AddToShipoutPictureBG*{\bcoverline{0.157}{0.78}{\bvolumeseries}}%
  \AddToShipoutPictureBG*{\bcoverset{0.2745}{22pt}{\bvolumetitle}}%
  \AddToShipoutPictureBG*{\bcoverset{0.330}{13pt}{\bvolumespan}}%
  \null\clearpage
  \nopagecolor\restoregeometry
}

\newcommand{\btitlepage}{%
  \thispagestyle{empty}%
  \begin{center}
    \vspace*{0.10\paperheight}
    {\large\scshape\bvolumeauthor}\par\vspace{2em}
    {\scshape\bvolumeseries}\par\vspace{4em}
    {\Huge\bvolumetitle}\par\vspace{1.5em}
    {\large\bvolumespan}\par\vfill
    \ifx\bvolumeedition\empty\else{\small\bvolumeedition}\par\fi
    \vspace{0.08\paperheight}
  \end{center}
  \clearpage
}

% ---------------------------------------------------------------------------
% The divisions.
% ---------------------------------------------------------------------------

\newcommand{\bchaptername}{CHAPTER}
\newcommand{\bexercisesname}{EXERCISES}
\newcommand{\bexercisesforname}{Exercises for}
\newcommand{\bhistoricalname}{HISTORICAL NOTE}
\newcommand{\bcontentsname}{CONTENTS}

% \blanguage sets the five words this class writes that are not in the corpus.
% Everything else on the page came out of content/ and is already in the right
% language, which is why this is five macros and not a message catalogue.
%
% "Exercises for" is the one of the five that is a phrase rather than a heading,
% and it is off the printed contents pages: the English Algebra I to III writes
% "Exercises for § 1" and Théories spectrales writes "Exercices du § 1".
\newcommand{\blanguage}[1]{%
  \def\btmp{#1}%
  \def\btmpfr{fr}\def\btmpvi{vi}%
  \ifx\btmp\btmpfr
    \renewcommand{\bchaptername}{CHAPITRE}%
    \renewcommand{\bexercisesname}{EXERCICES}%
    \renewcommand{\bexercisesforname}{Exercices du}%
    \renewcommand{\bhistoricalname}{NOTE HISTORIQUE}%
    \renewcommand{\bcontentsname}{TABLE DES MATIÈRES}%
  \fi
  \ifx\btmp\btmpvi
    \renewcommand{\bchaptername}{CHƯƠNG}%
    \renewcommand{\bexercisesname}{BÀI TẬP}%
    \renewcommand{\bexercisesforname}{Bài tập cho}%
    \renewcommand{\bhistoricalname}{GHI CHÚ LỊCH SỬ}%
    \renewcommand{\bcontentsname}{MỤC LỤC}%
  \fi
}

\newcommand{\bchaptertitle}{}
\newcommand{\bchapternumeral}{}
\newcommand{\bsectionlabel}{}

\newcommand{\bchapter}[4]{%
  \normalsize
  \cleardoublepage
  \renewcommand{\bchaptertitle}{#2}%
  \renewcommand{\bchapternumeral}{#1}%
  \renewcommand{\bsectionlabel}{}%
  \thispagestyle{plain}%
  \phantomsection\label{#4}%
  \addcontentsline{toc}{chapter}{\bchaptername\ #1. #3}%
  \vspace*{0.06\paperheight}
  {\centering\large\scshape\bchaptername\ #1\par\vspace{1.2em}
   \LARGE #2\par}
  \vspace{2.5em}
  \markboth{#3}{\brhead{#3}{}}%
}

% \bsection is a § or an appendix. The heading is worked out in Go from the
% front matter rather than read off the body, because the body writes it in six
% different ways across the corpus and one of them lost its numeral to an OCR.
% The § head is set in capitals and upright, not in bold. That was read off the
% page: "§ 5. GROUPS OPERATING ON A SET" is roman caps a size up from the body,
% and the only bold on a Bourbaki page is the numbered subsection head under it.
\newcommand{\bsection}[3]{%
  \normalsize
  \phantomsection\label{#3}%
  \addcontentsline{toc}{section}{#1. #2}%
  \renewcommand{\bsectionlabel}{#1}%
  \vspace{2.2em}
  {\centering\large #1.\quad \MakeUppercase{#2}\par}
  \vspace{1.4em}
  \markright{\brhead{#2}{#1}}%
  \@afterindentfalse\@afterheading
}

\newcommand{\bunnumbered}[3]{%
  \normalsize
  \cleardoublepage
  \phantomsection\label{#3}%
  \addcontentsline{toc}{chapter}{#2}%
  \renewcommand{\bchapternumeral}{}%
  \renewcommand{\bsectionlabel}{}%
  \thispagestyle{plain}%
  \vspace*{0.06\paperheight}
  {\centering\LARGE #1\par}
  \vspace{2.5em}
  \markboth{#2}{\brhead{#2}{}}%
}

% \banchor is a label that a heading may or may not carry. Most headings in the
% corpus have one and some have none, and \label{} twice in a document is two
% labels with the same empty name, which LaTeX complains about once per pair and
% then forgets.
\newcommand{\banchor}[1]{\ifx\relax#1\relax\else\phantomsection\label{#1}\fi}

% \bno is a numbered subsection, the "no." Bourbaki divides a § into. The number
% goes in the margin the way the printing puts it, and the title is set in the
% caps the corpus already has it in.
%
% It goes in the contents, which is most of what the contents of a Bourbaki is.
% Page xi of the English Algebra I to III gives one line to chapter I and six to
% the numbered subsections of its § 1, and a contents that stopped at the §§
% would be four pages of the wrong book.
%
% The same title turns up in three places in three different forms, so three of
% the five arguments are that title. The head on the page is capitals, the
% contents line is sentence case, and the running head is title case set as small
% capitals. They are three strings rather than one string set three ways:
% content/ has only the capitals, and lower casing them and capitalising the
% first letter gets a quarter of them wrong, which is what Hensel, Zariski,
% Noetherian and every capital letter that is a mathematical symbol do to a rule
% like that. The sentence case comes out of manifests/toc/, which is a reading of
% the volume's own contents pages. Where there is none the head goes in its place.
%
% The fourth argument arrives as a \bheadfit of the long form and the short one,
% because whether the long one fits is a question about font metrics.
\newcommand{\bno}[5]{%
  \par\vspace{1.4em}
  \banchor{#5}%
  \addcontentsline{toc}{subsection}{\protect\bnoline{#1}{#3}}%
  \noindent{\small\bfseries #1.\ \MakeUppercase{#2}}\par
  \vspace{0.5em}
  \markright{\brhead{#4}{\bsectionlabel.#1}}%
  \@afterindentfalse\@afterheading
}

% \bfigure stands where the printing has a figure and this build has not got it.
%
% The corpus is text and stays text: no image is committed to it and none ever
% will be. So a figure is represented by what the reading wrote down about it,
% which is the alt text of the Markdown image, set in a frame so that a reader
% can see at once that something is missing rather than reading a stray caption
% as if it were a sentence of the book.
%
% The frame is what an honest gap looks like. Printing the caption on its own,
% which is what happened before, tells the reader nothing and reads as prose.
\newcommand{\bfigure}[1]{%
  \par\vspace{0.9em}
  \begingroup\centering
  \fbox{\begin{minipage}{0.75\linewidth}\centering\small\itshape #1\end{minipage}}%
  \par\endgroup
  \vspace{0.9em}
  \noindent\ignorespaces
}

% \bfnmark is the footnote mark on a numbered subsection title.
%
% It is a name of our own for \footnotemark and not \footnotemark itself,
% because the title goes through \MakeUppercase in \bno above, \MakeUppercase
% expands what it is given, and \footnotemark is not robust: it expands to an
% \@ifnextchar looking for the optional argument, and inside that expansion the
% error is "Use of \@xfootnotemark doesn't match its definition". Declaring the
% mark robust puts \protect in front of it, which is what \MakeUppercase honours,
% so the mark reaches the page unexpanded and the uppercasing leaves it alone.
%
% One title in the corpus needs this, no. 10 of Integration VI, § 2.
\DeclareRobustCommand{\bfnmark}{\footnotemark}

% \bnoline is a contents line for a numbered subsection. The number is set right
% against a fixed width so that the stop of "10." falls under the stop of "1.",
% which is what the printing does and what a left aligned number does not.
\DeclareRobustCommand{\bnoline}[2]{\makebox[1.6em][r]{#1.}\hspace{0.45em}#2}

% \bnamed is a subsection with no number, which a few files have where the
% printing runs two together.
\newcommand{\bnamed}[2]{%
  \par\vspace{1.4em}
  \banchor{#2}%
  \noindent{\normalsize\bfseries #1}\par
  \vspace{0.5em}
  \@afterindentfalse\@afterheading
}

% \bpart is a heading the corpus writes inside a body where it is not the file
% naming itself. Four files have one.
\newcommand{\bpart}[2]{%
  \par\vspace{1.6em}
  \banchor{#2}%
  \noindent{\large\bfseries #1}\par\vspace{0.6em}
  \@afterindentfalse\@afterheading
}

% ---------------------------------------------------------------------------
% Statements.
% ---------------------------------------------------------------------------

% \bstate is the run-in head of a Proposition, a Definition, a Lemma and the
% five other kinds, set the way the printing sets them: the kind in small caps,
% the number after it, an em rule, and then the statement, which the corpus has
% already marked as italic.
%
% The fourth argument is the permanent tag, the four characters that have named
% this statement since M4 and that are the same four in every language. It goes
% in the margin, small and grey, so that a reader with the PDF and a reader with
% the site are looking at the same object and can say so out loud. The fifth is
% the anchor, which is what a cross reference elsewhere in the volume points at
% and is a different thing from the tag: the tag is permanent and the anchor is
% this language's slug.
\newcommand{\btag}[1]{%
  \ifx\relax#1\relax\else
    \marginpar{\hfill\scriptsize\ttfamily\color{gray}#1}%
  \fi
}

\newcommand{\bstate}[5]{%
  \par\vspace{0.9em}
  \banchor{#5}%
  \noindent\textsc{#1}%
  \ifx\relax#2\relax\else\ \textsc{#2}\fi
  \ifx\relax#3\relax\else\ (#3)\fi
  .\,---\,\btag{#4}\nobreak\ignorespaces
}

% The printing sets the statement of a proposition or a definition in italic and
% the discussion that follows it upright, so these bracket the one paragraph the
% renderer decided was the statement. They are two commands rather than a group
% because the statement is a paragraph of its own and a group that spans a \par
% reads worse than a pair that says where it starts and where it stops.
\newcommand{\bstatebegin}{\itshape\ignorespaces}
\newcommand{\bstateend}{\normalfont}

% \bproof is the body of a proof, which the corpus does not mark and which is
% here for the day it does.
\newcommand{\bproof}{\par\vspace{0.5em}\noindent\emph{Proof.}\ \ignorespaces}

% ---------------------------------------------------------------------------
% Exercises.
% ---------------------------------------------------------------------------

% The exercises of a whole chapter are gathered at its end under one head, and
% not run after each § as an earlier draft of this class had them. The corpus
% keeps them one file to an exercise under the § they belong to, so both
% arrangements were available and the printing had to be looked at. It gathers
% them: printed page 123 of the English Algebra I to III is the last page of
% § 10 of chapter I, page 124 opens EXERCISES, and under that head come "§ 1"
% through "§ 10" in order, each a centred line with its own run of numbers. The
% contents lists them the same way, ten "Exercises for § N" lines together after
% the last § of the chapter.
%
% They are set at the size of the body and not a size down, which was measured
% rather than assumed and which corrects what this class did before. The
% baselines of the exercise pages of that volume come out 11.0 pt apart and the
% median word height is 8.82 pt, and both numbers are what the body pages give.
% The French volumes do set their exercises smaller; this one does not.
\newcommand{\bexercises}{%
  \clearpage
  \thispagestyle{plain}%
  \vspace*{0.02\paperheight}
  {\centering\large\bexercisesname\par}
  \vspace{1.6em}
  \markboth{\bexercisesname}{\bexercisesname}%
  \setlength{\parindent}{1.2em}%
}

% \bexercisesfor opens the run of exercises belonging to one §. On the page it
% is the § label alone, centred, the way the printing sets it. In the contents
% it is a full line at the § indent, "Exercises for § 1", which is why the label
% is passed rather than composed out of \bsectionlabel: by the time this runs
% the chapter is over and \bsectionlabel holds the last § of it.
\newcommand{\bexercisesfor}[2]{%
  \par\vspace{1.6em}
  \phantomsection\label{#2}%
  \addcontentsline{toc}{section}{\bexercisesforname\ #1}%
  {\centering #1\par}
  \vspace{0.8em}
  \@afterindentfalse\@afterheading
}

% The star marks the ones the printing sets as supplementary, and it is the
% corpus's own mark rather than this class's invention.
%
% The number is set roman and followed by a full stop at the head of an indented
% paragraph, which is how page 124 of that volume has it. It was bold and hung
% into the margin here before, which is a house style and not this house's.
\newcommand{\bexercise}[3]{%
  \par\vspace{0.5em}
  \banchor{#3}%
  \noindent\hspace{\parindent}#1.\ %
  \ifx\relax#2\relax\else$*$\fi\ignorespaces
}

% ---------------------------------------------------------------------------
% Running heads, contents, and the rest of the furniture.
% ---------------------------------------------------------------------------

% The head and the foot were read off the page rather than guessed, and they are
% not the arrangement a LaTeX book gives you. On a verso the chapter numeral
% sits alone in the outer corner and the chapter title is centred in small caps.
% On a recto the title of the numbered subsection is centred in small caps and
% the outer corner carries the section and subsection together, "§ 5.1". The
% page number is at the foot on the outer side on both, not in the head.
%
% This is written out as \@oddhead and \@evenhead rather than through fancyhdr,
% which was tried first and dropped one field per side: with fancyhdr loaded, a
% [LE] head printed nothing on a verso and a [RO] head nothing on a recto, while
% the centre field and both feet printed. Four macros of plain LaTeX do the same
% job with nothing to go wrong in, so that is what is here.
%
% \brhead is what goes in the recto mark. It carries two fields, which is one
% more than \markright has room for, so the mark holds a call to it and the head
% expands the mark and gets both. The zero width boxes put the corner items on
% the trim edge while leaving the title centred on the measure, which is how the
% printing has it.
% \bheadfont is the size the head is set at, which is 9 pt against a 10 pt body,
% in the tracked face declared above. Both numbers are measurements.
%
% The tracking is calibrated against the reference volume, Springer Nature's 2023
% typesetting of Algebra Chapter 8, the one book in the sources that is set rather
% than photographed. Page 60 of it carries "MULTIPLICITIES AND LENGTHS IN
% SEMISIMPLE MODULES" in the head, running from 90.31 to 328.09, so 237.78 pt.
% The same words in lmromancaps10 small capitals at 9 pt measure 231.06 pt set
% solid, 234.84 pt tracked 1 and 238.62 pt tracked 2, so tracked 2 lands 0.8 pt
% from the printing on a line of 238 pt.
%
% Reading the width as a size instead gives nothing usable: 205.38 pt at 8 pt and
% 231.06 pt at 9 pt bracket the target, and 8.5 pt tracked 2 gives 225.36 pt,
% still 12 pt short. The printing is tracking a 9 pt face, not setting a larger
% one solid.
%
% This was 5.5 and that number came from a different face and a different page.
% It was fitted to "DISTRIBUTIVITY OF ONE INTERNAL LAW WITH RESPECT TO ANOTHER"
% spanning 262 pt in the 1998 photographic reprint of Algebra I to III, set in
% TeX Gyre Termes, whose small capitals are narrower than these. Carried over to
% Latin Modern it overshoots the reference by 14 pt, and it was overshooting on a
% measure that has since come in by 64 pt.
%
% Small capitals of a title case string, not of a capitalised one. Feeding the
% capitals the corpus stores to \scshape gives full capitals throughout, which
% measures 362.93 pt for that same head, 88 pt past the measure. That single
% mistake was 201 of the 364 overfull boxes the English volume reported: every
% head longer than about forty characters ran off the page.
\DeclareRobustCommand{\bheadfont}{\bheadface\fontsize{9}{10.8}\selectfont\scshape}

% \bheadfit sets the first argument if it fits the head and the second if it does
% not. The printing shortens a head that will not go rather than letting it run,
% and it shortens at a full stop: the subsection called "Symmetric algebra of a
% direct sum. Symmetric algebra of a free module. Symmetric algebra of a graded
% module" carries a head that reads "SYMMETRIC ALGEBRA OF A DIRECT SUM" and
% nothing more. It shortens only what has to be shortened, though. "Annihilators.
% Faithful modules. Monogenous modules" is three sentences and prints whole.
%
% The choice is made here rather than in Go because the thing being asked is how
% wide a string sets in this font at this size, and TeX is holding the metrics.
% Go passes both forms and this picks one.
%
% Four subsections of the English Algebra have a title with no full stop in it
% that is still too long, so there is nowhere to shorten and the third branch
% squeezes. The worst of the four is 33 pt over 281 pt, which is a scale of 0.89
% and not a thing a reader notices, and the alternative is a head that runs 14 pt
% past the trim. Two of the four are titles the OCR mangled rather than titles
% that are really that long, and the audit lists all four.
%
% Robust, all four of these, because the head reaches the page through \markright
% and \markright expands its argument as it writes the mark. A \settowidth in the
% middle of that expansion is a \setbox where TeX wants a token, and the first
% version of this was fragile: every long head printed in full and none of the
% shortening ran at all.
\newlength{\bheadwd}
\DeclareRobustCommand{\bheadfit}[2]{%
  \settowidth{\bheadwd}{\bheadfont #1}%
  \ifdim\bheadwd>\bheadroom
    \settowidth{\bheadwd}{\bheadfont #2}%
    \ifdim\bheadwd>\bheadroom
      \resizebox{\bheadroom}{!}{\bheadfont #2}%
    \else
      \bheadfont #2%
    \fi
  \else
    \bheadfont #1%
  \fi
}
% The room a head has is the measure less the two corner labels and a gap on
% each side. The head carries a locator at one end and a folio at the other, and
% a folio reading A VIII.243 is the widest either corner gets.
%
% 7 em, which is 70 pt at body size, leaves 263.9 pt of the 333.9 pt measure. The
% reference sets its own head in that band: its title runs 90.31 to 328.09 on a
% text block that starts at 52.7, so it clears the inner corner by 37.6 pt and
% the outer by 58.5 pt, and a title centred on the measure with the tighter of
% those two gaps at each end would have 258.7 pt to work in. This was 9 em while
% the measure was 397.9 pt wide, and holding 9 em through the move to the
% Springer page would have cut the room by 31 per cent at the same time as the
% face got wider, which is a lot of heads squeezed for no reason.
\newlength{\bheadroom}
\AtBeginDocument{\setlength{\bheadroom}{\dimexpr\textwidth-7em\relax}}

% \brhead is what goes in the recto mark: the locator at the inner edge and the
% title between it and the folio. The locator is at body size and upright, which
% is measured: "3.5" prints 12.36 pt wide and the digits of this font are half
% an em, so it is set at 10 pt and not at the 9 pt the head around it uses.
%
% The folio is not here. It is added by the page style, because the head of a
% recto and the head of a verso want it at opposite ends and the mark is written
% once for both.
\DeclareRobustCommand{\brhead}[2]{%
  \makebox[0pt][l]{\normalsize\upshape #2}\hfil\bheadfont #1\hfil
}
\makeatletter
% The head carries the folio and the foot is empty, which is what the reference
% page does and what nearly every mathematical book in print does. The old style
% put the folio at the foot and the locator at the outer edge of the head, which
% is the 1970 Hermann arrangement.
%
% Three slots, mirrored about the spine: the locator at the inner edge, the head
% of the section between, the page number at the outer edge. Both corners are
% set in boxes of no width so that neither takes anything off the room the title
% is centred in.
\newcommand{\ps@bourbaki}{%
  \let\@mkboth\markboth
  \def\@oddhead{\rightmark\makebox[0pt][r]{\normalsize\upshape\thepage}}%
  \def\@evenhead{\makebox[0pt][l]{\normalsize\upshape\thepage}%
    \hfil\bheadfont\leftmark\hfil
    \makebox[0pt][r]{\normalsize\upshape\bchapternumeral}}%
  \def\@oddfoot{}%
  \def\@evenfoot{}%
}
% The opening page of a chapter, of the contents and of a historical note takes
% its number at the foot and no running head, which is what the printing does.
\newcommand{\ps@bourbakiplain}{%
  \let\@mkboth\markboth
  \def\@oddhead{}%
  \def\@evenhead{}%
  \def\@oddfoot{\hfil\small\thepage}%
  \def\@evenfoot{\small\thepage\hfil}%
}
\let\ps@plain\ps@bourbakiplain
\makeatother
\pagestyle{bourbaki}

% The contents follows the printing rather than the book class, which sets a
% chapter line in bold with no leader and then indents a section line under it in
% a way this page does not. Page xi of the English Algebra I to III has three
% levels, all one size, all with dot leaders and a right aligned page number: the
% chapter and the pieces of front matter flush left in small caps, the §§ a step
% in under them in sentence case, and the numbered subsections a step in again
% with their numbers right aligned. There is air above a chapter line and a
% little above each § block, and none between the subsections of a §.
\makeatletter
% The leaders are tighter than LaTeX's and the page number column is wider. Both
% were read off page xi: the dots there run at about half the spacing
% \@dotsep gives by default, and the widest number in the book, a four figure
% page, has to sit clear of the longest title without the two touching, which
% \@pnumwidth at 1.55em does not manage.
\renewcommand{\@dotsep}{2.5}
\renewcommand{\@pnumwidth}{2.2em}
\renewcommand{\@tocrmarg}{3.4em}
\renewcommand{\l@chapter}[2]{%
  \addpenalty{-\@highpenalty}%
  \vskip 1.1em \@plus\p@
  \@dottedtocline{0}{0em}{0em}{\scshape #1}{#2}%
  \penalty\@highpenalty
}
\renewcommand{\l@section}[2]{%
  \vskip 0.55em \@plus\p@
  \@dottedtocline{1}{1.5em}{0em}{#1}{#2}%
}
\renewcommand{\l@subsection}{\@dottedtocline{2}{3.3em}{2.05em}}
\makeatother

% Two levels below the chapter, which is the depth the printing goes to and one
% more than the book class shows by default. Without this the \addcontentsline
% in \bno writes every subsection into the .toc file and \@dottedtocline throws
% all of them away, which is a contents that is silently short.
\setcounter{tocdepth}{2}

% The contents has no title of its own. \tableofcontents sets one through
% \chapter*, which gives a page an inch of bold sans above the first entry, and
% the printing has nothing there: page xi carries the word CONTENTS in the
% running head, in the same place and the same face as ALGEBRAIC STRUCTURES on a
% body page, and the first entry drops from there. So this sets the marks and
% pours the .toc file out directly.
\makeatletter
\newcommand{\bcontents}{%
  \cleardoublepage
  \markboth{\bcontentsname}{\brhead{\bcontentsname}{}}%
  \vspace*{0.06\paperheight}
  \@starttoc{toc}%
  \cleardoublepage
}
\makeatother

% A display in the corpus is a formula on its own line and nothing else, so the
% spacing round it is the spacing of a formula in a paragraph rather than the
% spacing of a theorem.
%
% The comment that used to sit here said these were LaTeX's own values and they
% were not: LaTeX at ten point sets \abovedisplayskip to 10 pt and this had 6.
% Four points off every display in a book that is mostly displays is why a
% formula used to sit in the paragraph rather than stand out of it. These are
% the real ten point values.
\setlength{\abovedisplayskip}{10pt plus 2pt minus 5pt}
\setlength{\belowdisplayskip}{10pt plus 2pt minus 5pt}
\setlength{\abovedisplayshortskip}{0pt plus 3pt}
\setlength{\belowdisplayshortskip}{6pt plus 3pt minus 3pt}

% Ten point on 14.35, which is the leading of the reference page measured under
% \bpaper. LaTeX at ten point leads on twelve, so the spread is 14.35/12.
%
% The earlier value here was 0.917, ten on eleven, taken off twenty eight body
% pages of the 1998 reprint of the English Algebra I to III. It is a true
% measurement of that book and it is the wrong target, for the reason set out
% under \bpaper: that book is a photograph of a 1970 Hermann page.
\linespread{1.196}

\setlength{\parindent}{1.2em}
\setlength{\parskip}{0pt}
\frenchspacing
\emergencystretch=1.5em
\hbadness=10000
\vbadness=10000
\tolerance=2000

\endinput
