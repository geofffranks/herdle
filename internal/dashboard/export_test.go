package dashboard

import (
	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
)

// TicketTableForTest exposes ticketTable to the black-box _test package.
func (e Engine) TicketTableForTest(path string) []dticket { return e.ticketTable(path) }

// SyncNoteForTest / WipSyncForTest expose the sync helpers to the _test package.
func (e Engine) SyncNoteForTest(path, branch string) FlagNote { return e.syncNote(path, branch) }
func (e Engine) WipSyncForTest(path, branch string) (SyncState, string) {
	return e.wipSync(path, branch)
}

// TicketsForTest wraps plain tickets into the unexported dticket slice (EffLifecycle blank).
func (e Engine) TicketsForTest(ts []vcs.Ticket) []dticket {
	out := make([]dticket, len(ts))
	for i, t := range ts {
		out[i] = dticket{Ticket: t}
	}
	return out
}
func (e Engine) OpenPRRowsForTest(prs []vcs.PR, t []dticket, path string) []PRRow {
	return e.openPRRows(prs, t, path)
}
func (e Engine) MergedCleanupRowsForTest(prs []vcs.PR, t []dticket, path string) []MergedRow {
	return e.mergedCleanupRows(prs, t, path)
}

func (e Engine) WIPRowsForTest(r config.Resolved, prs []vcs.PR, t []dticket) []WIPRow {
	return e.wipRows(r, prs, t)
}
func (e Engine) WithLifecycleForTest(t dticket, lc string) dticket { t.EffLifecycle = lc; return t }

func (e Engine) UpNextRowsForTest(t []dticket) []UpNextRow     { return upNextRows(t) }
func (e Engine) ArtifactRowsForTest(path string) []ArtifactRow { return e.artifactRows(path) }
