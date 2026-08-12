package solve

import (
	"strings"
	"testing"
)

// Every case here is the real text of a real exercise, cut down to the sentences
// the rule turns on. The rule was got wrong twice on made up examples and right
// only after it was measured against the chapter, so the chapter is what it is
// tested on.

// s21ex12 is chapter VIII, § 21, exercise 12. It is the hard one: seven parts,
// an f printed "f )" in the middle of a paragraph, and three back-references in
// brackets that have the shape of a marker and are not one.
const s21ex12 = `Let H be a subgroup of G; suppose that for every $g\in G$- H, we have the equality $H\cap gHg^{-1}=\{1\}$.

a) Let $u$ be a central function on H. Prove that the function Ind$^G_H(u)$ coincides with $u$ on H $-\{1\}$.

b) Let $\lambda \in \widehat{H}$. Prove that $\theta_{\lambda}$ is the character of an irreducible representation of G (use a) to calculate $\langle \theta_{\lambda}, \theta_{\lambda}\rangle )$.

c) Set $\theta =\sum_{\lambda}d_{\lambda}\theta_{\lambda}$. Prove that we have $\theta (h) = 0$ if $h\in H-\{1\}$.

d) Let $L'$ be the set of elements of G that do not belong to any conjugates of H. Prove that L is a normal subgroup of G (deduce from c) that L is the kernel of a representation with character $\theta )$.

e) Prove that G is the semidirect product of H and L (calculate the order of L). f ) Let $h\in H-\{1\}$. Prove that the automorphism int($h$) restricted to L has no fixed point other than 1.

g) Prove that if the subgroup H has even order, then L is commutative (apply Exercise 23, d) of I, §6, p. 145).`

// s1ex3 is § 1, exercise 3, where b) opens no paragraph. It is the reason this
// cannot be a rule about the starts of lines.
const s1ex3 = `a) Give an example of a commutative field K and an isomorphism $\varphi$ from K to one of its subfield $K'$. Deduce that A is left Artinian and left Noetherian but neither right Artinian nor right Noetherian. b) Give an example of a module of finite length whose endomorphism ring is neither left Artinian nor left Noetherian.

c) Using the same method as in a), give an example of a left and right Artinian ring whose left length is different from its right length.`

// s16ex14 is § 16, exercise 14, whose c) is starred. The star is set as $*$ and
// it sits between the newline and the letter.
const s16ex14 = `Let A be a Noetherian commutative local ring and S an Azumaya algebra over A.

a) Let $e$ be an idempotent in S whose image in $S/\mathfrak{m}S$ is indecomposable.

b) Suppose that $\mathfrak{m}$ is nilpotent $*$(or, more generally, that A is complete).

$*$c) Assume from now on that A is complete. Let L be a separable extension of $\kappa$.

d) Suppose, moreover, that L is a Galois extension of $\kappa$, with Galois group G.

e) Prove that the homomorphism $H^q(\pi )$ is bijective for every $q\geqslant 1$.

f ) Conclude that the canonical homomorphisms Br(B$/A)\rightarrow$ Br(L$/\kappa )$ are bijective.`

// s2ex4 has no parts at all. It has a list of equivalent properties instead,
// numbered in roman, and the whole of it is one problem.
const s2ex4 = `Let A be a ring and $p, q$ two idempotents of A. Prove that the following properties are equivalent:

(i) We have $pq=qp$.

(ii) We have $u(e) = e$ for the endomorphism $u$ of A.

(iii) The element $p+q-pq$ is idempotent.`

func TestAnExerciseWithPartsSetInline(t *testing.T) {
	got := Parts(s21ex12)
	want := []string{"a", "b", "c", "d", "e", "f", "g"}
	if strings.Join(got, "") != strings.Join(want, "") {
		t.Errorf("Parts of § 21 exercise 12 = %v, want %v", got, want)
	}
	if got := Parts(s1ex3); strings.Join(got, "") != "abc" {
		t.Errorf("Parts of § 1 exercise 3 = %v, and its b) is mid paragraph", got)
	}
}

// The back-references are the whole difficulty. Every one of them follows a
// word or a comma, and if any were read as a marker the walk would take it in
// place of the real part and then run off the end of the letters.
func TestAReferenceToAPartIsNotAPart(t *testing.T) {
	for _, s := range []string{
		"a) First. Prove it (use a) to calculate the character).\n\nb) Second.",
		"a) First. Prove it.\n\nb) Second (deduce from a) that L is normal).",
		"a) First.\n\nb) Second (apply Exercise 23, d) of I, §6, p. 145).",
	} {
		if got := Parts(s); strings.Join(got, "") != "ab" {
			t.Errorf("Parts(%q) = %v, want a and b", s, got)
		}
	}
}

func TestAStarredPartIsAPart(t *testing.T) {
	got := Parts(s16ex14)
	if strings.Join(got, "") != "abcdef" {
		t.Errorf("Parts of § 16 exercise 14 = %v, and its c) is starred", got)
	}
}

// A one letter argument in brackets is not a part, and neither is a list of
// conditions numbered in roman. An exercise with no parts has to come back with
// none, since a judge asked for a line about part a of a part-less exercise
// will write one.
func TestAnExerciseWithNoParts(t *testing.T) {
	for _, s := range []string{
		s2ex4,
		"Let $u(e) = e$ and $v(a) = b$ for every $a$ in A. Prove that $u=v$.",
		"a) is the only marker here, and one part is not a decomposition.",
		"The parts begin at b) Prove this, which is not where a book starts.",
	} {
		if got := Parts(s); got != nil {
			t.Errorf("Parts(%q) = %v, want none", opening(s), got)
		}
	}
}

// The letters stop at h so that a list of equivalent conditions set in roman
// after the last part is not read as an i).
func TestTheLettersStopAtH(t *testing.T) {
	var b strings.Builder
	for c := 'a'; c <= 'j'; c++ {
		b.WriteString(string(c) + ") Prove it.\n\n")
	}
	if got := Parts(b.String()); strings.Join(got, "") != "abcdefgh" {
		t.Errorf("Parts = %v, want a through h", got)
	}
}

func opening(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
