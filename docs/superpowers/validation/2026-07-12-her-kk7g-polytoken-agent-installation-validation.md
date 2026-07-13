# Polytoken agent installation validation

## Herdle code review

- [x] Standard review completed
- [x] Standard review findings addressed
- [x] Deep review completed
- [x] Deep review findings addressed

Standard pass: I-2 fence-detector unification (fixed, gate bypass closed), I-1 hooks.json
re-marshal (accepted trade-off — spec says "parsed values" not "bytes"), I-3 ValidationReadOK
coupling (defensive comment added).
Deep pass: M-2 CRLF AGENTS.md false-positive (fixed), M-3 embedded-FS invariant test (added),
I-A/I-B write-tool/Bash obfuscation (pre-existing best-effort gate model, documented),
M-1 no-locking/M-4 no-rollback (accepted — low likelihood, conscious decision).
Review fix commit: a74913f.

## Automated

- [x] Focused Go suites pass
- [x] `make all` passes
- [x] Polytoken user configuration validates
- [x] Both Polytoken skills validate

## Human

- [x] Start/reload a Polytoken session and confirm the Herdle context is visible
- [x] Attempt each lifecycle transition and confirm the displayed denial/remediation is clear
