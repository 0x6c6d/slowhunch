#!/usr/bin/env bash
# Verification for T018 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "gitops tests" <<'CMD'
gotest ./internal/gitops/ '.*' TestPullRebase TestPushPlain TestPushFailed TestBehindCount TestRebaseConflictAborts TestPullFailedUnreachable
CMD
expect_ok "policy tests" <<'CMD'
gotest ./internal/policy/ '.*' TestNoForceGit TestGitOnlyGitops
CMD
expect_ok "methods exported" <<'CMD'
for m in HasRemote CurrentBranch HasUpstream Head PullRebase Push Behind; do
  grep -q -E -e "^func \(r \*Repo\) $m\(" internal/gitops/gitops.go || { echo "missing $m"; exit 1; }
done
CMD
expect_fail "negative: no fetch, reset or force literal in gitops" <<'CMD'
grep -n -e '"fetch"' -e '"reset"' -e '"--force' -e '"-f"' -e '"--hard"' internal/gitops/gitops.go
CMD
expect_fail "negative: planted --force literal in gitops fails TestNoForceGit" <<'CMD'
f=internal/gitops/zz_verify_neg.go
trap 'rm -f "$f"' EXIT
printf 'package gitops\n\nvar verifyNeg = "--force"\n' > "$f"
go test ./internal/policy/ -count=1 -run '^TestNoForceGit$'
CMD
expect_ok "planted file removed" <<'CMD'
test ! -e internal/gitops/zz_verify_neg.go
CMD
finish
