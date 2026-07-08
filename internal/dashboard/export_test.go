package dashboard

import (
	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
)

// TicketTableForTest exposes ticketTable to the black-box _test package.
func (e Engine) TicketTableForTest(path string) []dticket { return e.ticketTable(path) }

// SyncNoteForTest / WipSyncForTest expose the sync helpers to the _test package.
func (e Engine) SyncNoteForTest(path, remote, branch string) FlagNote {
	return e.syncNote(path, remote, branch)
}
func (e Engine) WipSyncForTest(path, remote, branch string) (SyncState, string) {
	return e.wipSync(path, remote, branch)
}

// TicketsForTest wraps plain tickets into the unexported dticket slice (EffLifecycle blank).
func (e Engine) TicketsForTest(ts []vcs.Ticket) []dticket {
	out := make([]dticket, len(ts))
	for i, t := range ts {
		out[i] = dticket{Ticket: t}
	}
	return out
}
func (e Engine) OpenPRRowsForTest(prs []vcs.PR, t []dticket, path, remote string) []PRRow {
	return e.openPRRows(prs, t, path, remote)
}
func (e Engine) MergedCleanupRowsForTest(prs []vcs.PR, t []dticket, path, remote string) []MergedRow {
	return e.mergedCleanupRows(prs, t, path, remote)
}

func (e Engine) WIPRowsForTest(r config.Resolved, prs []vcs.PR, t []dticket) []WIPRow {
	return e.wipRows(r, prs, t)
}
func (e Engine) WithLifecycleForTest(t dticket, lc string) dticket { t.EffLifecycle = lc; return t }

func (e Engine) UpNextRowsForTest(t []dticket) []UpNextRow     { return upNextRows(t) }
func (e Engine) ArtifactRowsForTest(path string) []ArtifactRow { return e.artifactRows(path) }

// SelectForgeForTest exposes the host->forge routing to the _test package,
// returning the resolved slug, forge kind ("github"/"gitlab"/""), and whether
// PR/MR features apply. It builds the routing from the engine's wired forges.
func (e Engine) SelectForgeForTest(r config.Resolved) (slug, kind string, ok bool) {
	_, slug, kind, ok = e.selectForge(r, e.routing())
	return slug, kind, ok
}

// ClassifyMergeForTest / MergeNoteForTest expose the merge-status helpers.
func ClassifyMergeForTest(pr vcs.PR) MergeStatus             { return classifyMerge(pr) }
func MergeNoteForTest(s MergeStatus, reason string) FlagNote { return mergeNote(s, reason) }

// PrCellForTest exposes the unexported prCell method to the black-box _test package.
func (e Engine) PrCellForTest(state PRState, allPRs []vcs.PR, tickets []dticket) PRCell {
	return e.prCell(state, allPRs, tickets)
}

// ProblemCountForTest exposes the unexported problemCount method to the black-box _test package.
func (e Engine) ProblemCountForTest(r config.Resolved, prs []vcs.PR, tickets []dticket) int {
	return e.problemCount(r, prs, tickets)
}
