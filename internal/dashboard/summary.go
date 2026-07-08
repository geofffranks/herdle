package dashboard

import (
	"sync"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
)

// maxSummaryConcurrency bounds how many projects are gathered at once. Each
// project's gather is dominated by a network round-trip to its forge (gh/glab),
// so concurrency collapses wall time from the sum of those calls to roughly the
// slowest one; the cap keeps the subprocess/connection count sane for users with
// many repos.
const maxSummaryConcurrency = 12

// Summary gathers one SummaryRow per configured project, in config file order,
// mirroring wip's summary(). Projects whose path does not exist on disk are
// skipped. The host->forge routing (GitHub via gh, GitLab via glab) is resolved
// once; PR cells degrade to "-" (PRNoSlug) when the project's forge CLI is absent
// or the remote belongs to no configured forge, so no "?" appears in those cases.
// When fetch is true each surviving project is git-fetched first (best-effort).
//
// Per-project gather runs concurrently (bounded by maxSummaryConcurrency): the
// work is network-bound and independent, so this is the difference between a
// snappy dashboard and one that waits on every forge call in series. Row order is
// preserved by writing into a pre-sized slice keyed by position.
func (e Engine) Summary(cfg *config.Config, fetch bool) (SummaryResult, error) {
	rt := e.routing()

	// Pre-filter to existing projects (cheap local stat), preserving config order.
	var projects []config.Project
	for _, p := range cfg.Projects {
		if e.dirExists(p.Path) {
			projects = append(projects, p)
		}
	}

	// Forge availability is a process-wide property (the CLI is on PATH or not), so
	// probe each wired forge once here rather than re-running Available() — an
	// exec.LookPath / stat — inside every project goroutine.
	forgeAvail := e.forgeAvailability()

	rows := make([]SummaryRow, len(projects))
	var (
		mu     sync.Mutex
		absent = map[string]bool{}
		wg     sync.WaitGroup
		sem    = make(chan struct{}, maxSummaryConcurrency)
	)
	for i, p := range projects {
		wg.Add(1)
		go func(i int, p config.Project) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if fetch {
				_ = e.Git.Fetch(p.Path)
			}
			r, _ := cfg.Resolve(p, e.Git) // error reserved for future hard failures
			client, slug, kind, isForge := e.selectForge(r, rt)
			avail := isForge && forgeAvail[kind]
			if isForge && !avail {
				mu.Lock()
				absent[forgeCLI(kind)] = true
				mu.Unlock()
			}
			// Fetch the ticket table once per project and feed both cells: prCell
			// needs it for the tk-validation reclassification, tkCell for its
			// in-progress/ready counts. Gated on HasTickets so a repo without a
			// .tickets dir spawns no `tk query` at all.
			present, _ := e.TK.HasTickets(p.Path)
			var tickets []dticket
			if present {
				tickets = e.ticketTable(p.Path)
			}
			// Fetch this project's PRs once (state "all") and feed the PR cell now
			// (problemCount reuses the same slice).
			var (
				allPRs  []vcs.PR
				prState = PRNoSlug
			)
			if isForge && avail && slug != "" {
				if got, err := client.PRList(slug, "all"); err != nil {
					prState = PRUnknown
				} else {
					allPRs, prState = got, PRCounted
				}
			}
			// Count problems only when the PR data is trustworthy. A repo with no forge
			// legitimately has no PRs, so its local WIP/cleanup problems are real and
			// should be counted. But a forge repo whose PRs we could not fetch — CLI
			// absent, unresolved slug, or a failed call (prState != PRCounted) — has a nil
			// PR list, which would make wipRows treat real PR branches as orphaned WIP and
			// inflate the count. Degrade to zero there (the PR cell already shows "-"/"?").
			problems := 0
			if !isForge || prState == PRCounted {
				problems = e.problemCount(r, allPRs, tickets)
			}
			rows[i] = SummaryRow{
				Name:     r.Name,
				Head:     e.head(p.Path),
				PR:       e.prCell(prState, allPRs, tickets),
				TK:       e.tkCell(p.Path, present, tickets),
				Problems: problems,
			}
		}(i, p)
	}
	wg.Wait()

	// Name the missing CLI(s) only when at least one project routes to a forge that
	// would otherwise show PR/MR counts; otherwise the note would be spurious.
	var absentForges []string
	for _, cli := range []string{"gh", "glab"} { // stable order
		if absent[cli] {
			absentForges = append(absentForges, cli)
		}
	}
	return SummaryResult{Rows: rows, AbsentForges: absentForges}, nil
}

// problemCount totals the flagged conditions the drilldown surfaces for one repo,
// excluding the merge-status classification (that is the summary's merge column).
// It reuses the drilldown row builders so the summary count equals the drilldown
// by construction. prs is the "all"-state list shared with prCell.
func (e Engine) problemCount(r config.Resolved, prs []vcs.PR, tickets []dticket) int {
	n := len(e.mergedCleanupRows(prs, tickets, r.Path, r.Remote))
	for _, w := range e.wipRows(r, prs, tickets) {
		if w.Problem != "" {
			n++
		}
	}
	for _, pr := range e.openPRRows(prs, tickets, r.Path, r.Remote) {
		// openPRRows always puts the merge-status note at Notes[0]; a PR is a problem
		// only when it *also* carries an actionable (>= SevYellow) sync or tk-validation
		// note. Dim SevNone notes (e.g. "<remote> only" for a branch not checked out
		// locally) are informational, not problems.
		for _, note := range pr.Notes[1:] {
			if note.Sev >= SevYellow {
				n++
				break
			}
		}
	}
	return n
}

// head mirrors wip's git_head.
func (e Engine) head(path string) HeadInfo {
	var h HeadInfo
	h.Branch, _ = e.Git.CurrentBranch(path) // "" => detached
	dirty, err := e.Git.IsDirty(path)
	// wip: `git diff --quiet && git diff --cached --quiet || dirty="*"`. A git
	// error (e.g. not a repo) trips the `||`, so treat an error as dirty.
	h.Dirty = err != nil || dirty
	if behind, ahead, err := e.Git.Divergence(path, "@{upstream}", "HEAD"); err == nil {
		h.Behind, h.Ahead = behind, ahead
	}
	return h
}

// prCell builds the summary PR cell from an already-fetched PR list (fetched with
// state "all" so the same slice can feed problemCount's merged-cleanup detection).
// state carries forge availability: PRNoSlug/PRUnknown pass straight through;
// PRCounted means count and classify the OPEN subset only.
func (e Engine) prCell(state PRState, allPRs []vcs.PR, tickets []dticket) PRCell {
	if state != PRCounted {
		return PRCell{State: state}
	}
	cell := PRCell{State: PRCounted}
	for _, pr := range allPRs {
		if pr.State != "OPEN" { // the "all" fetch also carries MERGED/CLOSED; the count is open-only
			continue
		}
		cell.Count++
		switch classifyMerge(pr) {
		case MergeReady:
			if _, bad := prTKIssue(tickets, pr.Number, pr.HeadRefName); bad {
				cell.Attention++ // forge says mergeable, but tk is not validated
			} else {
				cell.Ready++
			}
		case MergeConflicts, MergeChecksFailing, MergeChangesRequested, MergeBlocked:
			cell.Attention++
		}
	}
	return cell
}

// tkCell mirrors wip's `tk ls --status=in_progress | grep -c .` and
// `tk ready | grep -c '\[open\]'`: in-progress count, plus ready tickets that are
// also open (cross-referenced against Tickets' statuses).
// tkCell takes the already-fetched present flag and ticket table (built once per
// project in Summary) to avoid a second `tk query` per project. Closed tickets
// are filtered out of ticketTable, but tkCell never counted them anyway (only
// in_progress and open statuses), so the counts are unchanged.
func (e Engine) tkCell(path string, present bool, tickets []dticket) TKCell {
	if !present {
		return TKCell{Present: false}
	}
	cell := TKCell{Present: true}
	open := make(map[string]bool)
	for _, t := range tickets {
		switch t.Status {
		case "in_progress":
			cell.InProgress++
		case "open":
			open[t.ID] = true
		}
	}
	if ready, err := e.TK.Ready(path); err == nil {
		for _, id := range ready {
			if open[id] {
				cell.Ready++
			}
		}
	}
	return cell
}
