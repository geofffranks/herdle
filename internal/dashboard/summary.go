package dashboard

import "github.com/geofffranks/herdle/internal/config"

// Summary gathers one SummaryRow per configured project, in config file order,
// mirroring wip's summary(). Projects whose path does not exist on disk are
// skipped: wip's loader admits only existing dirs (`[ -d "$path" ]`), whereas
// herdle's config keeps every configured project, so skipping here is what makes
// `herdle --all` match `wip --all` row-for-row. When fetch is true each surviving
// project is git-fetched first (best-effort, like wip's `2>/dev/null`).
func (e Engine) Summary(cfg *config.Config, fetch bool) ([]SummaryRow, error) {
	var rows []SummaryRow
	for _, p := range cfg.Projects {
		if !e.dirExists(p.Path) {
			continue
		}
		if fetch {
			_ = e.Git.Fetch(p.Path)
		}
		r, _ := cfg.Resolve(p, e.Git) // error reserved for future hard failures
		rows = append(rows, SummaryRow{
			Name: r.Name,
			Head: e.head(p.Path),
			PR:   e.prCell(r.Slug),
			TK:   e.tkCell(p.Path),
		})
	}
	return rows, nil
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

// prCell mirrors wip's pr_count.
func (e Engine) prCell(slug string) PRCell {
	if slug == "" {
		return PRCell{State: PRNoSlug}
	}
	prs, err := e.GH.PRList(slug, "open")
	if err != nil {
		return PRCell{State: PRUnknown}
	}
	return PRCell{State: PRCounted, Count: len(prs)}
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
