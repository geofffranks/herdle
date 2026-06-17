package dashboard

import (
	"sync"

	"github.com/geofffranks/herdle/internal/config"
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
			rows[i] = SummaryRow{
				Name: r.Name,
				Head: e.head(p.Path),
				PR:   e.prCell(client, slug, isForge && avail),
				TK:   e.tkCell(p.Path),
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

// prCell mirrors wip's pr_count, extended for graceful degradation: when the
// project's forge CLI is absent or the project routes to no forge, the cell is
// PRNoSlug ("-") and the forge is not called. A forge project whose list call
// fails is PRUnknown ("?"). query is true only when a forge client is available
// and a slug resolved.
func (e Engine) prCell(client forgeClient, slug string, query bool) PRCell {
	if !query || client == nil || slug == "" {
		return PRCell{State: PRNoSlug}
	}
	prs, err := client.PRList(slug, "open")
	if err != nil {
		return PRCell{State: PRUnknown}
	}
	cell := PRCell{State: PRCounted, Count: len(prs)}
	for _, pr := range prs {
		switch classifyMerge(pr) {
		case MergeReady:
			cell.Ready++
		case MergeConflicts, MergeChecksFailing, MergeChangesRequested, MergeBlocked:
			cell.Attention++
		}
	}
	return cell
}

// tkCell mirrors wip's `tk ls --status=in_progress | grep -c .` and
// `tk ready | grep -c '\[open\]'`: in-progress count, plus ready tickets that are
// also open (cross-referenced against Tickets' statuses).
func (e Engine) tkCell(path string) TKCell {
	present, err := e.TK.HasTickets(path)
	// Mirror wip's `[ -d "$path/.tickets" ]`: any stat error is treated as absent.
	if err != nil || !present {
		return TKCell{Present: false}
	}
	cell := TKCell{Present: true}
	tickets, err := e.TK.Tickets(path)
	if err != nil {
		return cell // dir exists but tk failed -> 0/0
	}
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
