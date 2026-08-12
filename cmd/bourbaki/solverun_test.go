package main

import (
	"strings"
	"testing"
)

// A pilot asked for the first nine exercises of a § has to get the first nine.
// Lexically it gets 1, 10, 11, 12, 13, 14, 15, 2, 3, which is a set nobody
// chose and which reads in the report as though the § were nearly done.
func TestTheExercisesComeInTheOrderTheBookSetsThem(t *testing.T) {
	labels := []string{
		"alg-viii-s1-ex-10", "alg-viii-s2-ex-1", "alg-viii-s1-ex-2",
		"alg-viii-a1-ex-1", "alg-viii-s1-ex-1", "alg-viii-s10-ex-1",
	}
	inOrder(labels)
	want := "alg-viii-s1-ex-1 alg-viii-s1-ex-2 alg-viii-s1-ex-10 " +
		"alg-viii-s2-ex-1 alg-viii-s10-ex-1 alg-viii-a1-ex-1"
	if got := strings.Join(labels, " "); got != want {
		t.Errorf("in order:\n got %s\nwant %s", got, want)
	}
}

// The appendix numbers its sections from one, the same as the chapter, so a run
// asked for § 1 that got appendix 1 would report on 43 exercises of the wrong
// section.
func TestASectionAndAnAppendixOfTheSameNumber(t *testing.T) {
	cases := []struct {
		label, section string
		want           bool
	}{
		{"alg-viii-s1-ex-3", "1", true},
		{"alg-viii-s1-ex-3", "s1", true},
		{"alg-viii-s1-ex-3", "a1", false},
		{"alg-viii-a1-ex-3", "1", false},
		{"alg-viii-a1-ex-3", "a1", true},
		{"alg-viii-s21-ex-3", "2", false},
		{"alg-viii-s21-ex-3", "21", true},
		{"not a label", "1", false},
	}
	for _, c := range cases {
		if got := inSection(c.label, c.section); got != c.want {
			t.Errorf("inSection(%q, %q) = %v", c.label, c.section, got)
		}
	}
}
