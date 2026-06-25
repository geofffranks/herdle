package dashboard

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
)

// SyncState is the WIP "sync" column glyph: ✓ / ✗ / ·.
type SyncState int

const (
	SyncOK  SyncState = iota // local == remote
	SyncBad                  // differs / one-sided (reason -> Issue)
	SyncNA                   // no branch
)

// Severity drives flag/issue color in render; the engine classifies, render colors.
type Severity int

const (
	SevNone   Severity = iota // dim / neutral
	SevGreen                  // ✓ in sync
	SevYellow                 // ⚠ divergence / cleanup
	SevRed                    // ✗ / gap / dead
)

// FlagNote is a colored note cell (PR sync note, merged-cleanup flags).
type FlagNote struct {
	Text string
	Sev  Severity
}

type PRRow struct {
	Number int
	Head   string
	Title  string
	TKs    []string
	Notes  []FlagNote // merge status first; non-green sync note appended
}

type MergedRow struct {
	Number int
	Head   string
	Title  string
	Flags  FlagNote
}

type WIPRow struct {
	Lifecycle string
	Sync      SyncState
	TKID      string
	Branch    string
	Title     string
	Issue     string
	IssueSev  Severity
}

type UpNextRow struct {
	Lifecycle string
	TKID      string
	Title     string
	Priority  int
}

type ArtifactRow struct {
	TKID     string
	Kind     string
	Filename string
}

// Drilldown is the typed per-repo view; render turns it into wip-identical bytes.
type Drilldown struct {
	Name, Path string
	// Forge is the resolved forge kind for this repo: "github", "gitlab", or ""
	// (no forge). The renderer uses it to name the right CLI (gh vs glab) and noun
	// (PR vs MR) in section headers and degradation notes.
	Forge            string
	Fetched          bool
	Head             HeadInfo
	HasSlug          bool
	ForgeUnavailable bool
	ForgeAbsent      bool
	OpenPRs          []PRRow
	MergedCleanup    []MergedRow
	WIP              []WIPRow
	UpNext           []UpNextRow
	Artifacts        []ArtifactRow
}

// divFlag mirrors wip's div_flag: a local branch's divergence vs the given remote.
// Divergence returns (left, right) = (behind, ahead) for "remote/b...b".
func (e Engine) divFlag(path, remote, branch string) string {
	behind, ahead, err := e.Git.Divergence(path, remote+"/"+branch, branch)
	if err != nil {
		return "?"
	}
	switch {
	case ahead > 0 && behind > 0:
		return fmt.Sprintf("diverged ↑%d↓%d", ahead, behind)
	case ahead > 0:
		return fmt.Sprintf("↑%d unpushed", ahead)
	case behind > 0:
		return fmt.Sprintf("↓%d behind", behind)
	default:
		return ""
	}
}

// syncNote mirrors wip's sync_note for an open PR's head branch, against remote.
func (e Engine) syncNote(path, remote, branch string) FlagNote {
	if local, _ := e.Git.LocalBranchExists(path, branch); !local {
		name := remote
		if name == "" {
			name = "remote"
		}
		return FlagNote{Text: name + " only", Sev: SevNone}
	}
	if remote == "" {
		return FlagNote{Text: "⚠ no remote configured", Sev: SevYellow}
	}
	if r, _ := e.Git.RemoteBranchExists(path, remote, branch); !r {
		return FlagNote{Text: "⚠ local-only (not pushed)", Sev: SevYellow}
	}
	if d := e.divFlag(path, remote, branch); d != "" {
		return FlagNote{Text: "⚠ " + d, Sev: SevYellow}
	}
	return FlagNote{Text: "✓ in sync", Sev: SevGreen}
}

// wipSync mirrors wip's wip_sync for a non-PR branch, against remote. With no
// configured remote there is nothing to compare against, so the branch is n/a.
func (e Engine) wipSync(path, remote, branch string) (SyncState, string) {
	if remote == "" {
		return SyncNA, ""
	}
	local, _ := e.Git.LocalBranchExists(path, branch)
	remoteExists, _ := e.Git.RemoteBranchExists(path, remote, branch)
	switch {
	case local && remoteExists:
		if d := e.divFlag(path, remote, branch); d != "" {
			return SyncBad, d
		}
		return SyncOK, ""
	case local:
		return SyncBad, "local only — not pushed"
	case remoteExists:
		return SyncBad, "remote only — no local branch"
	default:
		return SyncNA, ""
	}
}

func (e Engine) openPRRows(prs []vcs.PR, tickets []dticket, path, remote string) []PRRow {
	var rows []PRRow
	for _, pr := range prs {
		if pr.State != "OPEN" {
			continue
		}
		notes := []FlagNote{mergeNote(classifyMerge(pr), pr.BlockReason)}
		if sync := e.syncNote(path, remote, pr.HeadRefName); sync.Sev != SevGreen {
			notes = append(notes, sync)
		}
		if text, bad := prTKIssue(tickets, pr.Number, pr.HeadRefName); bad {
			notes = append(notes, FlagNote{Text: "⚠ " + text, Sev: SevYellow})
		}
		rows = append(rows, PRRow{
			Number: pr.Number,
			Head:   pr.HeadRefName,
			Title:  pr.Title,
			TKs:    tksForPR(tickets, pr.Number, pr.HeadRefName),
			Notes:  notes,
		})
	}
	return rows
}

func (e Engine) mergedCleanupRows(prs []vcs.PR, tickets []dticket, path, remote string) []MergedRow {
	var rows []MergedRow
	for _, pr := range prs {
		if pr.State != "MERGED" {
			continue
		}
		var flags []string
		if ok, _ := e.Git.LocalBranchExists(path, pr.HeadRefName); ok {
			flags = append(flags, "⚠ local branch")
		}
		if remote != "" {
			if ok, _ := e.Git.RemoteBranchExists(path, remote, pr.HeadRefName); ok {
				flags = append(flags, "⚠ "+remote+" branch")
			}
		}
		if tks := tksForPR(tickets, pr.Number, pr.HeadRefName); len(tks) > 0 {
			flags = append(flags, "⚠ tk "+strings.Join(tks, ",")+" open")
		}
		if len(flags) == 0 {
			continue
		}
		rows = append(rows, MergedRow{
			Number: pr.Number, Head: pr.HeadRefName, Title: pr.Title,
			Flags: FlagNote{Text: strings.Join(flags, " · "), Sev: SevYellow},
		})
	}
	return rows
}

func (e Engine) glob(pattern string) ([]string, error) {
	if e.Glob != nil {
		return e.Glob(pattern)
	}
	return filepath.Glob(pattern)
}

func (e Engine) globHit(pattern string) bool {
	m, err := e.glob(pattern)
	return err == nil && len(m) > 0
}

// ticketTable returns the non-closed tickets, each annotated with its effective
// lifecycle. Mirrors wip's build_tk_table (closed skipped, lifecycle derived).
func (e Engine) ticketTable(path string) []dticket {
	tickets, _ := e.TK.Tickets(path)
	var out []dticket
	for _, t := range tickets {
		if t.Status == "closed" {
			continue
		}
		out = append(out, dticket{Ticket: t, EffLifecycle: e.effectiveLifecycle(path, t)})
	}
	return out
}

// effectiveLifecycle mirrors wip: a set lifecycle wins; else derive
// planned/designed from a spec/plan file embedding the id; else "-" for an
// explicit "-", "?" for an absent field (wip's `${lc:-?}`).
func (e Engine) effectiveLifecycle(path string, t vcs.Ticket) string {
	if t.Lifecycle != "" && t.Lifecycle != "-" {
		return t.Lifecycle
	}
	if e.globHit(filepath.Join(path, "docs/superpowers/plans", "*"+t.ID+"*")) {
		return "planned"
	}
	if e.globHit(filepath.Join(path, "docs/superpowers/specs", "*"+t.ID+"*")) {
		return "designed"
	}
	if t.Lifecycle == "-" {
		return "-"
	}
	return "?"
}

// excludedBranches is the config-driven set of branches kept out of the WIP
// section: the universal trunks + the configured remote name + the per-project
// base and integration branches. (De-personalized: no hardcoded dev/geoff-main.)
func (e Engine) excludedBranches(r config.Resolved) map[string]bool {
	ex := map[string]bool{"main": true, "master": true, "HEAD": true}
	if r.Remote != "" {
		ex[r.Remote] = true
	}
	if r.Base != "" {
		ex[r.Base] = true
	}
	if r.Integration != "" {
		ex[r.Integration] = true
	}
	return ex
}

func (e Engine) wipRows(r config.Resolved, prs []vcs.PR, tickets []dticket) []WIPRow {
	path := r.Path
	excluded := e.excludedBranches(r)

	localBranches, _ := e.Git.LocalBranches(path)
	gone := map[string]bool{}
	set := map[string]bool{}
	for _, b := range localBranches {
		set[b.Name] = true
		if b.UpstreamGone {
			gone[b.Name] = true
		}
	}
	if r.Remote != "" { // no configured remote -> nothing to list
		remoteBranches, _ := e.Git.RemoteBranches(path, r.Remote)
		for _, b := range remoteBranches {
			set[b] = true
		}
	}
	var names []string
	for b := range set {
		if excluded[b] || strings.HasPrefix(b, "backup/") {
			continue
		}
		names = append(names, b)
	}
	sort.Strings(names)

	inPR := map[string]bool{}
	for _, pr := range prs {
		inPR[pr.HeadRefName] = true
	}

	var rows []WIPRow
	matched := map[string]bool{}
	for _, b := range names {
		if inPR[b] || gone[b] {
			continue
		}
		sync, reason := e.wipSync(path, r.Remote, b)
		row := WIPRow{Branch: b, Sync: sync}
		if t, ok := tkForBranch(tickets, b); ok {
			matched[t.ID] = true
			row.Lifecycle = t.EffLifecycle
			row.TKID = t.ID
			row.Title = t.Title
		} else {
			row.Lifecycle = "-"
			row.Issue = "no tk"
		}
		if reason != "" {
			if row.Issue != "" {
				row.Issue += " · "
			}
			row.Issue += reason
		}
		if row.Issue != "" {
			if sync == SyncBad {
				row.IssueSev = SevRed
			} else {
				row.IssueSev = SevYellow
			}
		}
		rows = append(rows, row)
	}

	// standalone in-flight tks: in_progress, unmatched, not in a PR.
	for _, t := range tickets {
		if t.Status != "in_progress" || matched[t.ID] {
			continue
		}
		if tkInAnyPR(t, prs) {
			continue
		}
		row := WIPRow{Lifecycle: t.EffLifecycle, Sync: SyncNA, TKID: t.ID, Branch: "(no branch)", Title: t.Title}
		switch {
		case ghNum(t.ExternalRef) == "" && t.Branch == "":
			row.Issue, row.IssueSev = "no external-ref / branch", SevRed
		case t.Branch != "":
			row.Issue, row.IssueSev = "branch "+t.Branch+" missing", SevRed
		}
		rows = append(rows, row)
	}
	return rows
}

func readinessRank(lc string) int {
	switch lc {
	case "planned":
		return 0
	case "designed":
		return 1
	case "-":
		return 2
	default:
		return 3
	}
}

func upNextRows(tickets []dticket) []UpNextRow {
	type ranked struct {
		row  UpNextRow
		rank int
	}
	var rs []ranked
	for _, t := range tickets {
		if t.Status != "open" {
			continue
		}
		rs = append(rs, ranked{
			row:  UpNextRow{Lifecycle: t.EffLifecycle, TKID: t.ID, Title: t.Title, Priority: t.Priority},
			rank: readinessRank(t.EffLifecycle),
		})
	}
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].rank != rs[j].rank {
			return rs[i].rank < rs[j].rank
		}
		return rs[i].row.Priority < rs[j].row.Priority
	})
	out := make([]UpNextRow, len(rs))
	for i, x := range rs {
		out[i] = x.row
	}
	return out
}

// Drilldown gathers and classifies one repo's work state, mirroring wip's
// drilldown(). r supplies the de-personalized config (Name/Path/Slug/Base/
// Integration/Remote/RemoteHost). Forge degradation is handled transparently:
// ForgeAbsent is set when the forge CLI is unavailable, ForgeUnavailable when it
// is up but PRList errors.
func (e Engine) Drilldown(r config.Resolved, fetch bool) (Drilldown, error) {
	rt := e.routing()
	client, slug, kind, isForge := e.selectForge(r, rt)
	avail := isForge && client.Available()
	// ForgeAbsent notes that PR/MR data is hidden because the forge CLI is missing
	// — only relevant when this project routes to a forge; a repo with no forge
	// shows no PR sections regardless, so the note would otherwise be spurious.
	d := Drilldown{Name: r.Name, Path: r.Path, Forge: kind, HasSlug: isForge, ForgeAbsent: isForge && !avail}

	if fetch {
		_ = e.Git.Fetch(r.Path)
		d.Fetched = true
	} else if r.Remote != "" {
		_ = e.Git.PruneRemote(r.Path, r.Remote)
	}

	d.Head = e.head(r.Path)

	var prs []vcs.PR
	if avail {
		if got, err := client.PRList(slug, "all"); err != nil {
			d.ForgeUnavailable = true
		} else {
			prs = got
		}
	}

	tickets := e.ticketTable(r.Path)

	d.OpenPRs = e.openPRRows(prs, tickets, r.Path, r.Remote)
	d.MergedCleanup = e.mergedCleanupRows(prs, tickets, r.Path, r.Remote)
	d.WIP = e.wipRows(r, prs, tickets)
	d.UpNext = upNextRows(tickets)
	d.Artifacts = e.artifactRows(r.Path)

	return d, nil
}

// ticketIDs lists the repo's tk ids from the .tickets/<id>.md filenames. These
// are the only strings recognized as ids in artifact filenames, so a slug that
// merely looks id-shaped (e.g. "movable-grou") is never mistaken for one. This
// generalizes wip's hardcoded `dr-[a-z0-9]{4}` grep to the repo's real prefix.
func (e Engine) ticketIDs(path string) []string {
	matches, _ := e.glob(filepath.Join(path, ".tickets", "*.md"))
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, strings.TrimSuffix(filepath.Base(m), ".md"))
	}
	return ids
}

// artifactID returns the real tk id embedded in an artifact filename — the
// left-most one that appears — or "" when none does (rendered as "-").
func artifactID(filename string, ids []string) string {
	best, bestAt := "", -1
	for _, id := range ids {
		if at := strings.Index(filename, id); at >= 0 && (bestAt < 0 || at < bestAt) {
			best, bestAt = id, at
		}
	}
	return best
}

func (e Engine) artifactRows(path string) []ArtifactRow {
	ids := e.ticketIDs(path)
	var rows []ArtifactRow
	for _, kind := range []string{"specs", "plans", "validation"} {
		matches, _ := e.glob(filepath.Join(path, "docs/superpowers", kind, "*.md"))
		sort.Strings(matches)
		for _, m := range matches {
			fn := filepath.Base(m)
			rows = append(rows, ArtifactRow{TKID: artifactID(fn, ids), Kind: kind, Filename: fn})
		}
	}
	return rows
}
