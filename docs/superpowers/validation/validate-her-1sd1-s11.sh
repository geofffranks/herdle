#!/usr/bin/env bash
# Validation for her-1sd1 (S11: README + install/usage/contributor docs + drift guard).
# Exercises the machine-checkable acceptance steps; prints ok:/FAIL: per check.
set -uo pipefail
cd "$(git rev-parse --show-toplevel)"
fail=0

echo "== make test (drift guard + full suite) =="
if make test >/tmp/her-1sd1-test.log 2>&1; then echo "ok: make test"; else echo "FAIL: make test (see /tmp/her-1sd1-test.log)"; fail=1; fi

echo "== command reference entries all run =="
make build >/dev/null 2>&1
for c in version "project add" "project set" "project rm" "project list" init doctor; do
  if ./herdle $c --help >/dev/null 2>&1; then echo "ok: herdle $c"; else echo "FAIL: herdle $c"; fail=1; fi
done

echo "== install asset names match release.yml matrix =="
grep -q 'herdle-${{ matrix.goos }}-${{ matrix.goarch }}' .github/workflows/release.yml \
  || { echo "FAIL: asset naming template missing in release.yml"; fail=1; }
for tuple in linux:amd64 linux:arm64 darwin:amd64 darwin:arm64 windows:amd64; do
  goos=${tuple%:*}; goarch=${tuple#*:}
  if grep -q "herdle-$goos-$goarch" docs/install.md \
     && grep -q "goos: $goos" .github/workflows/release.yml \
     && grep -q "goarch: $goarch" .github/workflows/release.yml; then
    echo "ok: $goos-$goarch"
  else echo "FAIL: $goos-$goarch"; fail=1; fi
done

echo "== README internal links resolve =="
grep -oE '\]\(([^)]+)\)' README.md | sed -E 's/\]\(([^)]+)\)/\1/' | grep -vE '^https?:' | while read -r link; do
  [ -e "$link" ] && echo "ok: $link" || { echo "FAIL: $link"; exit 1; }
done || fail=1

echo "== README has no construction/pre-release banner =="
if grep -qiE "Early development|Not yet usable|🚧" README.md; then echo "FAIL: banner present"; fail=1; else echo "ok: no banner"; fi

echo "== docs reference only the real config path/extension =="
if grep -q "config.yaml" docs/usage.md docs/install.md docs/configuration.md; then echo "FAIL: stray config.yaml reference"; fail=1; else echo "ok: config.toml everywhere"; fi

if [ "$fail" -eq 0 ]; then echo "ALL VALIDATION CHECKS PASSED"; else echo "SOME VALIDATION CHECKS FAILED"; fi
exit $fail
