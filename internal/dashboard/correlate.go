package dashboard

import (
	"regexp"
	"strconv"
	"strings"

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
// token in branch, so 59 matches "fix/59-x" but not "fix/590-x". It scans for the
// token directly instead of compiling a regexp per call: branchHasNum runs in the
// O(tickets×branches) and O(tickets×PRs) correlation loops, where a fresh
// regexp.MustCompile each call was pure overhead (n is always a digit run from
// ghNum, so the old regexp.QuoteMeta was a no-op).
func branchHasNum(branch, n string) bool {
	if n == "" {
		return false
	}
	for from := 0; ; {
		i := strings.Index(branch[from:], n)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(n)
		leftOK := start == 0 || !isASCIIDigit(branch[start-1])
		rightOK := end == len(branch) || !isASCIIDigit(branch[end])
		if leftOK && rightOK {
			return true
		}
		from = start + 1
	}
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

// ticketMatchesPR is wip's tks_for_pr / standalone-tk correlation predicate:
// a ticket correlates to a PR when its ghNum equals the PR number, the PR head
// branch carries the ghNum, or an explicit ticket branch equals the PR head.
// n is the ticket's precomputed ghNum; prNum is the PR number as a string.
func ticketMatchesPR(t dticket, n, prNum, prHead string) bool {
	return (n != "" && n == prNum) || branchHasNum(prHead, n) || (t.Branch != "" && t.Branch == prHead)
}

// dticketsForPR returns the tickets correlated to a PR, carrying their effective
// lifecycle. tksForPR is the id-only projection of this.
func dticketsForPR(tickets []dticket, prNum int, prHead string) []dticket {
	num := strconv.Itoa(prNum)
	var out []dticket
	for _, t := range tickets {
		if ticketMatchesPR(t, ghNum(t.ExternalRef), num, prHead) {
			out = append(out, t)
		}
	}
	return out
}

// tksForPR mirrors wip's tks_for_pr: ids of tickets correlated to a PR.
func tksForPR(tickets []dticket, prNum int, prHead string) []string {
	corr := dticketsForPR(tickets, prNum, prHead)
	if len(corr) == 0 {
		return nil // preserve the nil (not []string{}) the suite asserts on
	}
	out := make([]string, 0, len(corr))
	for _, t := range corr {
		out = append(out, t.ID)
	}
	return out
}

// prTKIssue reports whether an open PR's tk correlation fails to resolve to a
// validated ticket, plus the human text describing why. text/false when every
// correlated ticket is validated. A PR with no correlated ticket is only an
// issue when the table is non-empty: an empty table means this repo has no tk
// system, so a PR without a ticket is not a validation gap to surface.
func prTKIssue(tickets []dticket, prNum int, prHead string) (string, bool) {
	corr := dticketsForPR(tickets, prNum, prHead)
	if len(corr) == 0 {
		if len(tickets) > 0 { // repo uses tk, this PR is uncorrelated
			return "no tk", true
		}
		return "", false
	}
	var frags []string
	for _, t := range corr {
		switch t.EffLifecycle {
		case "validated":
			// resolved — contributes no fragment
		case "?", "-", "": // "" never occurs from ticketTable; guarded for synthetic fixtures
			frags = append(frags, "tk "+t.ID+" unvalidated ("+t.EffLifecycle+")")
		default:
			frags = append(frags, "tk "+t.ID+" "+t.EffLifecycle)
		}
	}
	if len(frags) == 0 {
		return "", false
	}
	return strings.Join(frags, ", "), true
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
