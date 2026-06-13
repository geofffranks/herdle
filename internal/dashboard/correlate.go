package dashboard

import (
	"regexp"
	"strconv"

	"github.com/geofffranks/herdle/internal/vcs"
)

// dticket is a non-closed ticket annotated with its effective lifecycle (the
// designed/planned-from-disk derivation the vcs.Ticket doc defers to consumers).
type dticket struct {
	vcs.Ticket
	EffLifecycle string
}

var digitsRe = regexp.MustCompile(`[0-9]+`)

// ghNum mirrors wip's gh_num: the last run of digits in a tk external-ref.
// "gh-682" -> "682", "/issues/59" -> "59", "" -> "".
func ghNum(ref string) string {
	m := digitsRe.FindAllString(ref, -1)
	if len(m) == 0 {
		return ""
	}
	return m[len(m)-1]
}

// branchHasNum mirrors wip's br_has_num: n (non-empty) appears as a whole-number
// token in branch, so 59 matches "fix/59-x" but not "fix/590-x".
func branchHasNum(branch, n string) bool {
	if n == "" {
		return false
	}
	re := regexp.MustCompile(`(^|[^0-9])` + regexp.QuoteMeta(n) + `([^0-9]|$)`)
	return re.MatchString(branch)
}

// ticketMatchesPR is wip's tks_for_pr / standalone-tk correlation predicate:
// a ticket correlates to a PR when its ghNum equals the PR number, the PR head
// branch carries the ghNum, or an explicit ticket branch equals the PR head.
// n is the ticket's precomputed ghNum; prNum is the PR number as a string.
func ticketMatchesPR(t dticket, n, prNum, prHead string) bool {
	return (n != "" && n == prNum) || branchHasNum(prHead, n) || (t.Branch != "" && t.Branch == prHead)
}

// tksForPR mirrors wip's tks_for_pr: ids of tickets correlated to a PR.
func tksForPR(tickets []dticket, prNum int, prHead string) []string {
	num := strconv.Itoa(prNum)
	var out []string
	for _, t := range tickets {
		if ticketMatchesPR(t, ghNum(t.ExternalRef), num, prHead) {
			out = append(out, t.ID)
		}
	}
	return out
}

// tkForBranch mirrors wip's WIP inner loop: the first ticket whose explicit
// branch equals b, or whose ghNum appears in b.
func tkForBranch(tickets []dticket, branch string) (dticket, bool) {
	for _, t := range tickets {
		if (t.Branch != "" && t.Branch == branch) || branchHasNum(branch, ghNum(t.ExternalRef)) {
			return t, true
		}
	}
	return dticket{}, false
}

// tkInAnyPR mirrors wip's standalone-tk filter: does any PR correlate to this
// ticket (same predicate shape as tksForPR, ticket-centric). A ticket with no
// ghNum and no branch correlates to nothing.
func tkInAnyPR(t dticket, prs []vcs.PR) bool {
	n := ghNum(t.ExternalRef)
	if n == "" && t.Branch == "" {
		return false
	}
	for _, pr := range prs {
		if ticketMatchesPR(t, n, strconv.Itoa(pr.Number), pr.HeadRefName) {
			return true
		}
	}
	return false
}
