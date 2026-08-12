package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The rows come back in the order the lanes finished, which is two hosts of
// different speeds racing down one list. The table is read against the book, so
// it is sorted the way the book sets the exercises and not the way the fleet
// returned them.
func TestTheReviewTableReadsInBookOrder(t *testing.T) {
	rows := []reviewRow{
		{Label: "alg-viii-s1-ex-10"}, {Label: "alg-viii-a1-ex-1"},
		{Label: "alg-viii-s1-ex-2"}, {Label: "alg-viii-s2-ex-1"},
		{Label: "alg-viii-s1-ex-1"},
	}
	sortRows(rows)
	var got []string
	for _, r := range rows {
		got = append(got, r.Label)
	}
	want := "alg-viii-s1-ex-1 alg-viii-s1-ex-2 alg-viii-s1-ex-10 " +
		"alg-viii-s2-ex-1 alg-viii-a1-ex-1"
	if strings.Join(got, " ") != want {
		t.Errorf("the table reads\n %s\nwant %s", strings.Join(got, " "), want)
	}
}

// A row stands when the judges say now what the file has said all along. The
// count of what changed is the whole point of the command, and a row that
// compared anything but the two statuses would report a re-judging that agreed
// as a re-judging that did not.
func TestARowStandsOnlyWhenTheStatusIsTheSame(t *testing.T) {
	cases := []struct {
		row  reviewRow
		want bool
	}{
		{reviewRow{Was: corpus.StatusVerified, Now: corpus.StatusVerified,
			Truth: "pass", Audit: "pass"}, true},
		{reviewRow{Was: corpus.StatusVerified, Now: corpus.StatusUnverified}, false},
		{reviewRow{Was: corpus.StatusBlocked, Now: corpus.StatusBlocked, Judged: false}, true},
		{reviewRow{Was: corpus.StatusPartial, Now: corpus.StatusVerified}, false},
	}
	for _, c := range cases {
		if got := c.row.agreed(); got != c.want {
			t.Errorf("was %s now %s: agreed %v", c.row.Was, c.row.Now, got)
		}
	}
}
