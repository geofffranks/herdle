package dashboard

import "github.com/geofffranks/herdle/internal/vcs"

// MergeStatus classifies an open PR's merge readiness. classifyMerge applies the
// rules in priority order; the first match wins.
type MergeStatus int

const (
	MergeNeutral          MergeStatus = iota // draft / checks running / mergeability not yet computed
	MergeReady                               // no conflicts, checks green or absent, no changes requested
	MergeConflicts                           // mergeable == CONFLICTING
	MergeChecksFailing                       // a blocking status check failed
	MergeChangesRequested                    // reviewDecision == CHANGES_REQUESTED
	MergeBlocked                             // not ready for a named non-hard reason (BlockReason): approval, rebase, threads, CI not yet passed
)

// rollupState is the reduction of a PR's statusCheckRollup.
type rollupState int

const (
	rollupNone    rollupState = iota // no checks reported
	rollupPassing                    // all checks complete and non-failing
	rollupPending                    // at least one check still running, none failing
	rollupFailing                    // at least one check failed
)

// classifyMerge maps an open PR to a MergeStatus.
func classifyMerge(pr vcs.PR) MergeStatus {
	if pr.IsDraft {
		return MergeNeutral
	}
	if pr.Mergeable == "CONFLICTING" {
		return MergeConflicts
	}
	rollup := mergeRollup(pr)
	if rollup == rollupFailing {
		return MergeChecksFailing
	}
	if pr.ReviewDecision == "CHANGES_REQUESTED" {
		return MergeChangesRequested
	}
	// A named non-hard blocker (GitLab's not_approved / need_rebase /
	// discussions_not_resolved / ci_must_pass). Checked before "ready": GitLab only
	// reports mergeable once these clear, so the two are mutually exclusive, but
	// the explicit ordering keeps a blocked MR from ever reading as ready.
	if pr.BlockReason != "" {
		return MergeBlocked
	}
	if pr.Mergeable == "MERGEABLE" && (rollup == rollupPassing || rollup == rollupNone) {
		return MergeReady
	}
	return MergeNeutral
}

// mergeRollup reduces statusCheckRollup: any failure → failing, else any pending
// → pending, else passing when non-empty, else none.
func mergeRollup(pr vcs.PR) rollupState {
	if len(pr.StatusCheckRollup) == 0 {
		return rollupNone
	}
	anyPending := false
	for _, c := range pr.StatusCheckRollup {
		switch checkOutcome(c) {
		case rollupFailing:
			return rollupFailing
		case rollupPending:
			anyPending = true
		}
	}
	if anyPending {
		return rollupPending
	}
	return rollupPassing
}

// checkOutcome maps one rollup element to failing/pending/passing. A StatusContext
// carries State; a CheckRun carries Status/Conclusion.
func checkOutcome(c vcs.CheckRun) rollupState {
	if c.State != "" { // StatusContext
		switch c.State {
		case "FAILURE", "ERROR":
			return rollupFailing
		case "PENDING", "EXPECTED":
			return rollupPending
		default: // SUCCESS
			return rollupPassing
		}
	}
	if c.Status != "COMPLETED" { // CheckRun still running (QUEUED / IN_PROGRESS)
		return rollupPending
	}
	switch c.Conclusion {
	case "FAILURE", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "STARTUP_FAILURE":
		return rollupFailing
	default: // SUCCESS, NEUTRAL, SKIPPED, STALE
		return rollupPassing
	}
}

// mergeNote renders a MergeStatus as a colored note segment. The glyph+text live
// here (engine builds text, render applies color from Sev) — same split the sync
// notes already use. reason supplies the specific blocker text for MergeBlocked
// (e.g. "needs approval"); it is ignored for every other status.
func mergeNote(s MergeStatus, reason string) FlagNote {
	switch s {
	case MergeReady:
		return FlagNote{Text: "✓ ready to merge", Sev: SevGreen}
	case MergeConflicts:
		return FlagNote{Text: "✗ conflicts", Sev: SevRed}
	case MergeChecksFailing:
		return FlagNote{Text: "✗ checks failing", Sev: SevRed}
	case MergeChangesRequested:
		return FlagNote{Text: "✎ changes requested", Sev: SevYellow}
	case MergeBlocked:
		if reason == "" {
			reason = "not ready"
		}
		return FlagNote{Text: "⚠ " + reason, Sev: SevYellow}
	default:
		return FlagNote{Text: "—", Sev: SevNone}
	}
}
