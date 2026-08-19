Two English passages of a Bourbaki volume. Say where they state different
mathematics.

The first is the book. The second was made by translating the first into another
language and then translating that back, by two models that did not see each
other's work and did not see the book.

The two will not read alike and they are not supposed to. A round trip changes
every sentence it touches. Word order moves, a clause becomes two, a comma
becomes a semicolon, therefore becomes hence, a term gets a different English
name. None of that is a finding. If you report wording you will bury the one
thing this is for.

What it is for is the sentence that came back meaning something else. That is
the one failure the machine checks around this cannot see: the mathematics, the
tags, the headings, the citations and the block counts are all held span by span
elsewhere and they all pass on a fluent sentence that says the opposite of the
book. You are the only thing looking at the meaning.

Report a difference when, and only when, a mathematician reading the second
passage would come away believing something different from what the first
passage says. Specifically:

- a hypothesis on a result is missing, added, or weakened, and this is the most
  important one. "Let A be a ring" where the book says "Let A be a commutative
  ring" is a finding. So is a finite dropped, a non-empty dropped, a continuous
  dropped.
- a quantifier changed, or two quantifiers swapped order. Every for some, there
  exists for for all, the order of a for all and a there exists.
- a statement reversed, or an implication turned round. If P then Q where the
  book says if Q then P. A necessary condition called sufficient.
- a number, an index, a degree or a dimension that differs.
- a cross reference that points somewhere else. § 3 for § 2, Theorem 4 for
  Theorem 1, Chapter II for Chapter III.
- something the book states that is not in the second passage at all, or a
  mathematical claim in the second passage that the book does not make.

Do not report:

- any difference of wording, register, spelling, punctuation or paragraphing.
- a term given a different English name, as long as it is the same object.
  Mapping for map, one to one for injective, and so on.
- a formula written differently that means the same thing.
- a sentence of narration or motivation that is shorter, longer or gone, as long
  as no mathematical claim went with it.
- anything about which passage reads better.

Where you are unsure whether a difference is mathematical, report it and say in
the reason why you were unsure. A false report costs somebody a minute of
reading. A dropped hypothesis nobody reports goes into a book.

Answer as JSON and nothing else. No fence, no preface.

{"same": true, "differences": []}

or

{"same": false, "differences": [
  {"kind": "hypothesis", "english": "the words from the first passage",
   "back": "the words from the second", "why": "one sentence"}
]}

kind is one of statement, hypothesis, quantifier, number, reference, omission,
addition.

=====THE BOOK=====
{{ENGLISH}}
=====WHAT CAME BACK=====
{{BACK}}
=====
