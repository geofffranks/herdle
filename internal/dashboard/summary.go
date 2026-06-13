package dashboard

import "github.com/geofffranks/herdle/internal/config"

// Summary gathers one SummaryRow per configured project, in config file order,
// mirroring wip's summary(). Projects whose path does not exist on disk are
// skipped. gh availability and the known GitHub hosts are resolved once; PR cells
// degrade to "-" (PRNoSlug) when gh is absent or the remote is not a GitHub host,
// so no "?" appears in those cases. When fetch is true each surviving project is
// git-fetched first (best-effort).
func (e Engine) Summary(cfg *config.Config, fetch bool) (SummaryResult, error) {
	ghAvail := e.GH.Available()
	known := e.knownGitHubHosts()
	var rows []SummaryRow
	anyGitHub := false
	for _, p := range cfg.Projects {
		if !e.dirExists(p.Path) {
			continue
		}
		if fetch {
			_ = e.Git.Fetch(p.Path)
		}
		r, _ := cfg.Resolve(p, e.Git) // error reserved for future hard failures
		slug, isGitHub := effectiveSlug(r, known)
		if isGitHub {
			anyGitHub = true
		}
		rows = append(rows, SummaryRow{
			Name: r.Name,
			Head: e.head(p.Path),
			PR:   e.prCell(slug, isGitHub, ghAvail),
			TK:   e.tkCell(p.Path),
		})
	}
	// Note gh-absence only when at least one project is a GitHub remote that would
	// otherwise show PR counts; with no GitHub projects the note would be spurious.
	return SummaryResult{Rows: rows, GHAbsent: !ghAvail && anyGitHub}, nil
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

// prCell mirrors wip's pr_count, extended for graceful degradation: when gh is
// absent or the project has no GitHub remote, the cell is PRNoSlug ("-") and gh
// is not called. A GitHub project whose gh call fails is PRUnknown ("?").
func (e Engine) prCell(slug string, isGitHub, ghAvail bool) PRCell {
	if !ghAvail || !isGitHub || slug == "" {
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
