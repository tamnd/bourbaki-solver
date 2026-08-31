% bourbaki.cls sets a volume of the Elements the way the printing sets it.
%
% Everything here was measured off a scan rather than guessed. The yellow is
% #FEC746 and it is that on both of the two French Springer volumes that were
% sampled, over 85 per cent of the cover pixels on each. The blue of the lettering
% is about #0077B4, noisier because the cover is a JPEG inside the PDF. The four
% bands of the cover sit at the same fractions of the trim height on two volumes
% of different trim, which is what says they are fractions and not millimetres.
%
% The trim comes from manifests/books.yaml, which took it from the PDF, so a
% built Algebra I is 128 by 199.5 mm because that is what the book on the shelf
% is. Nothing here has a default paper size worth using.
%
% One thing this does not do is reproduce the Springer logo, which is somebody
% else's mark and not part of the mathematics.

\NeedsTeXFormat{LaTeX2e}
\ProvidesClass{bourbaki}[2026/08/26 A volume of the Elements de mathematique]

\LoadClass[10pt,twoside]{book}

% no-math because the mathematics is set by newtxmath below and not by fontspec.
% Without it fontspec claims the math families for itself and newtxmath's
% declarations are overwritten, which shows up as Computer Modern digits inside
% a Times formula.
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
% newtxmath is the mathematics of the Times the text is set in. It is the whole
% reason the page holds together: amssymb and mathrsfs on their own are Computer
% Modern, and a Computer Modern formula in the middle of a Times paragraph reads
% as two books printed on the same sheet.
%
% unicode-math with STIX Two Math was tried first and abandoned, and the reason
% is worth writing down because it will be tried again otherwise. unicode-math
% defines every symbol as a \Umathchar primitive. hyperref, when it turns a
% heading into a PDF bookmark string, throws the math shift away and then tries
% to expand what was inside it, and a \Umathchar is not something it can expand.
% The build died on a real heading, SS 4 of chapter II of Algebre, whose title
% carries \otimes, with "Improper alphabetic constant". newtxmath's symbols are
% ordinary \mathchar definitions and hyperref has digested those for thirty
% years.
%
% The corpus uses 283 distinct control sequences inside its math spans. All 283
% were probed for existence under the old stack and under this one. The two
% stacks miss exactly the same four and no others, so nothing that used to set
% has stopped setting. \widecheck is one of the four and has a fallback below.
% The other three were not commands at all: \hook, \lvertarrow and \sqleftarrow
% were what a reading made of a sign it could not name. They have since been
% read off the printed pages and replaced in the corpus by what is actually
% there, so the probe now misses one, not four.
\RequirePackage{newtxmath}
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
% -1 sitting above it. A reading names it \overparen or \wideparen, after the
% two packages that carry it, and neither package can be loaded here: yhmath
% takes over \widehat and mathabx takes over most of the symbols. newtxmath has
% the sign under a third name, so the two readings are pointed at it rather than
% a package pulled in for one accent. \overgroup was the other candidate and is
% wrong: it draws a flat bar with the ends turned down, and the printing has a
% curve.
\providecommand{\overparen}[1]{\widearc{#1}}
\providecommand{\wideparen}[1]{\widearc{#1}}

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

% Times, which is what a Springer mathematics volume is set in.
%
% This was Latin Modern, and Latin Modern is Computer Modern: it is the face a
% document is set in when nobody chose one. Springer does not set the Elements
% in it and does not set anything else in it either. A Springer mathematics
% volume, the Graduate Texts and the Universitext and the Grundlehren alike, is
% set in a Times, and so is nearly every journal a reader of this book has read.
% Times is a darker and narrower face than Computer Modern, it holds ink at ten
% point in a way Computer Modern's thin strokes do not, and it is the reason a
% Springer page reads as a printed book and a LaTeX default reads as a preprint.
%
% TeX Gyre Termes is that Times. It is the free cut of it, it is in tectonic's
% bundle, and it is the face newtxmath's letters were drawn against, so the
% italic in a formula and the italic in a sentence are the same italic. STIX Two
% Text and TeX Gyre Pagella were set beside it on the same page of real Bourbaki
% before this was chosen. STIX is the same Times with a slightly tighter fit and
% would have done as well for the prose, but its math font is unicode-math only
% and that stack does not survive hyperref, which is written out above. Pagella
% is a Palatino and is not what Springer uses.
%
% Vietnamese was the condition Latin Modern was chosen for and Termes meets it.
% Tiếng Việt, Đại số, Bổ đề, Định lý, Mệnh đề, Hệ quả and the whole of Latin
% Extended Additional set in it with no missing character. Two of the four
% languages this builds stack tone marks on vowels and a font that gets those
% wrong cannot set half the corpus.
%
% The small caps are real. \textsc is what every statement head in the book
% uses, and TeX Gyre Termes carries the feature in the regular file, so there is
% no separate caps file to name here and no fontspec warning about faking them.
% Latin Modern needed lmromancaps10 named explicitly for that reason.
%
% The fonts are named by file rather than by family. tectonic ships them inside
% its bundle rather than installing them, so \setmainfont{TeX Gyre Termes} fails
% on a machine that has no TeX Live and works on one that has, which is exactly
% the kind of difference this build is meant not to have. The file names resolve
% out of the bundle on any machine, and every one below was checked before it
% was written down.
\setmainfont{texgyretermes-regular.otf}[
  BoldFont       = texgyretermes-bold.otf,
  ItalicFont     = texgyretermes-italic.otf,
  BoldItalicFont = texgyretermes-bolditalic.otf,
]
% TeX Gyre Heros is the Helvetica that pairs with a Times, and nothing in the
% book is set in it: the running heads are small capitals of the text face and
% the corpus has no sans in it. It is here so that a document that ever asks for
% one does not fall back to Computer Modern Sans beside a Times page.
\setsansfont{texgyreheros-regular.otf}[
  BoldFont       = texgyreheros-bold.otf,
  ItalicFont     = texgyreheros-italic.otf,
  BoldItalicFont = texgyreheros-bolditalic.otf,
]
% TeX Gyre Cursor is the Courier of the same family. Its whole use is the four
% character permanent tag in the margin, set at 7 pt in grey.
\setmonofont{texgyrecursor-regular.otf}[
  BoldFont   = texgyrecursor-bold.otf,
  ItalicFont = texgyrecursor-italic.otf,
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
\newfontfamily\bheadface{texgyretermes-regular.otf}[LetterSpace=5.5]

% The roman inside a formula. newtxmath sets the letters and the symbols but
% takes its upright alphabet and its digits from whatever the kernel has, and
% with fontspec in no-math mode the kernel still has Computer Modern. Left
% alone, every digit, every parenthesis and every \operatorname{Card} in the
% book comes out Computer Modern in the middle of a Times formula. pdffonts on
% a test page showed four Latin Modern faces embedded before these lines and
% none after them.
%
% qtm, qhv and qcr are the Type1 TeX Gyre Termes, Heros and Cursor, which are
% the same three faces named above in the encoding the math machinery wants.
\DeclareSymbolFont{operators}{T1}{qtm}{m}{n}
\SetSymbolFont{operators}{bold}{T1}{qtm}{b}{n}
\DeclareMathAlphabet{\mathrm}{T1}{qtm}{m}{n}
\DeclareMathAlphabet{\mathbf}{T1}{qtm}{b}{n}
\DeclareMathAlphabet{\mathit}{T1}{qtm}{m}{it}
\DeclareMathAlphabet{\mathsf}{T1}{qhv}{m}{n}
\DeclareMathAlphabet{\mathtt}{T1}{qcr}{m}{n}

% The scripts a Times has nothing for. The corpus holds 239 characters of
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
% It is a Times-metric serif, so a Greek word inside an English sentence sits at
% the same weight and the same x-height as the sentence around it rather than
% announcing itself.
%
% book/unicode.go decides what goes inside this, and it wraps whole runs rather
% than single characters so that no word is set in two faces.
\IfFontExistsTF{FreeSerif.otf}{%
  \newfontfamily\bfallbackfont{FreeSerif.otf}[
    BoldFont       = FreeSerifBold.otf,
    ItalicFont     = FreeSerifItalic.otf,
    BoldItalicFont = FreeSerifBoldItalic.otf,
  ]%
  \newcommand{\bscript}[1]{{\bfallbackfont #1}}
}{%
  \newcommand{\bscript}[1]{#1}%
}
% A bookmark is a string and not a page, so a face has no meaning in one. Left
% alone hyperref tries to expand \bfallbackfont into the outline and writes a
% warning for every heading that quotes a word in another alphabet. This says
% the wrapper is the identity there, which keeps the word in the bookmark and
% drops only the face. It is the same reason the mathematics is set by newtxmath
% and not by unicode-math, written up at the top of this file.
\pdfstringdefDisableCommands{\let\bscript\@firstofone}

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
% newtxmath brings its own extension font, txexs, so this should now never fire.
% It stays because the cost of a line is nothing and the cost of that failure
% was a volume that would not build in one language and would in another.
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

% \bpaper is the trim out of the manifest. The margins are proportional to it
% rather than fixed, because the volumes run from 128 mm wide to 174 mm and a
% margin that suits one leaves the other either cramped or half empty.
%
% The fractions are the printing's own, off eight pages of the English Algebra I
% to III taken from four places in the volume. The text block runs from 18.5 pt
% to 344.7 pt on a 363.12 pt page every time, so the two margins are 0.051 of the
% trim each and the measure is 0.898 of it. That is a narrow margin by the
% standards of a modern book and it is what makes a Bourbaki page look like one.
% The first version of this had 0.090 and 0.075, which cost 22 pt of measure and
% ran the English Algebra to 127 per cent of the printed page count.
%
% Vertically the block runs 46 pt to 511 pt with the head at 18 pt and the folio
% at 529 pt, which is what the four fractions below give.
%
% The margin note column has to live inside the 18.5 pt, because there is no more
% room than the printing left. It carries a four character tag at 7 pt in a
% monospaced face, which measures 14.7 pt, so 0.042 of the trim holds it and 0.006
% keeps it off the text. includemp=false because geometry's default is to count
% that column as part of the body: left at the default it took another 28 pt off
% the measure for a column the printing does not have.
\newcommand{\bpaper}[2]{%
  \geometry{paperwidth=#1,paperheight=#2,%
    inner=0.051\paperwidth,outer=0.051\paperwidth,%
    top=0.078\paperheight,bottom=0.098\paperheight,%
    headsep=0.018\paperheight,footskip=0.040\paperheight,%
    includemp=false,%
    marginparwidth=0.042\paperwidth,marginparsep=0.006\paperwidth}%
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
% Page 29 of the English Algebra I to III carries "DISTRIBUTIVITY OF ONE
% INTERNAL LAW WITH RESPECT TO ANOTHER" in the head and it spans 262 pt. Set
% solid in TeX Gyre Termes small capitals the same words measure 211.68 pt at
% 8 pt, 238.14 pt at 9 pt and 264.60 pt at 10 pt. Reading that as a size gives
% 10 pt, and a running head the same size as the body is not what the page
% shows: the printed head is plainly smaller than the text under it.
%
% The width is made up with tracking instead, which is what the printing is
% actually doing. At 9 pt the same string measures 255.78 pt tracked 4.0,
% 258.65 pt tracked 4.65 and 262.39 pt tracked 5.5, so 5.5 lands 0.4 pt from
% the printing on a line of 262 pt while the head keeps the size the page says
% it has. Latin Modern hid this because its small capitals are far wider than a
% Times: it gave 262.68 pt at 8 pt set solid, so the tracking got absorbed into
% a wrong size and the head came out a size too small ever since.
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
% The room a head has is the measure less the corner label and a gap, which is
% what the printing leaves: the head on page 29 stops at 305 pt and the "§ 3.5"
% runs from 310 pt to the edge of the block.
\newlength{\bheadroom}
\AtBeginDocument{\setlength{\bheadroom}{\dimexpr\textwidth-4.5em\relax}}

% \brhead is what goes in the recto mark. The corner label is at body size and
% upright, which is also measured: "3.5" prints 12.36 pt wide and the digits of
% this font are half an em, so it is set at 10 pt and not at the 9 pt the head
% around it uses.
\DeclareRobustCommand{\brhead}[2]{%
  \hfil\bheadfont #1\hfil\makebox[0pt][r]{\normalsize\upshape #2}%
}
\makeatletter
\newcommand{\ps@bourbaki}{%
  \let\@mkboth\markboth
  \def\@oddhead{\rightmark}%
  \def\@evenhead{\makebox[0pt][l]{\normalsize\upshape\bchapternumeral}%
    \hfil\bheadfont\leftmark\hfil}%
  \def\@oddfoot{\hfil\small\thepage}%
  \def\@evenfoot{\small\thepage\hfil}%
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
% spacing of a theorem. These are LaTeX's own values, set here so that a change
% is one line and not a search.
\setlength{\abovedisplayskip}{6pt plus 2pt minus 2pt}
\setlength{\belowdisplayskip}{6pt plus 2pt minus 2pt}
\setlength{\abovedisplayshortskip}{3pt plus 1pt minus 1pt}
\setlength{\belowdisplayshortskip}{4pt plus 1pt minus 1pt}

% The printing sets ten point on eleven. That was measured rather than guessed:
% the baselines of twenty eight body pages of the English Algebra I to III come
% out 11.0 pt apart, and LaTeX at ten point leads on twelve. Nine per cent of a
% thousand page volume is worth one line here.
\linespread{0.917}

\setlength{\parindent}{1.2em}
\setlength{\parskip}{0pt}
\frenchspacing
\emergencystretch=1.5em
\hbadness=10000
\vbadness=10000
\tolerance=2000

\endinput
