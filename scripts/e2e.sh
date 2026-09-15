#!/usr/bin/env bash
# End-to-end: two simulated machines sharing a local bare remote.
# Only touches .dev/e2e inside this implementation's directory.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
BIN=$ROOT/bin/dwkt
DEV=$ROOT/.dev/e2e
rm -rf "$DEV"
mkdir -p "$DEV"
git init -q --bare "$DEV/remote.git"
unset XDG_CONFIG_HOME XDG_DATA_HOME

as() {
	local m=$1
	shift
	DWKT_HOSTNAME=$m DWKT_CONFIG_DIR=$DEV/$m/config DWKT_DATA_DIR=$DEV/$m/data "$BIN" "$@"
}
fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok   $*"; }
expect() { # expect <needle> <haystack> <label>
	grep -qF -- "$1" <<<"$2" || fail "$3: expected '$1' in:"$'\n'"$2"
}
wait_bg() { sleep 1; } # let detached background pushes finish before asserting remote state

# --- init two machines on one bare remote
as a init "$DEV/remote.git" >/dev/null
as b init "$DEV/remote.git" >/dev/null
[[ -f $DEV/a/config/config.toml && -d $DEV/b/data/.git ]] || fail "init did not create config/data"
pass "init a and b on one bare remote"

# --- add parses inline tags, defaults to week + work
out=$(as a add "rerun QC #adni")
id=$(awk '{print $2}' <<<"$out")
line=$(cat "$DEV"/a/data/ops/a-*.jsonl)
expect '"title":"rerun QC"' "$line" "stored title"
expect '"tags":["adni"]' "$line" "stored tag"
expect '"on_week":true' "$line" "on week"
expect '"category":"work"' "$line" "category"
pass 'add "rerun QC #adni" → title "rerun QC", tag adni, on week, work'

# --- add on a, sync, visible on b
as a sync >/dev/null
out=$(as b ls)
expect "rerun QC #adni" "$out" "b sees a's task"
expect "Work" "$out" "grouped under Work"
pass "a add + sync → b ls shows it"

# --- concurrent writes on both, repeated syncs: never a merge conflict
as a add "from a" >/dev/null
as b add "from b" -c personal >/dev/null
as b tag rerun QC +fromb >/dev/null
as a edit "$id" --title "rerun QC v2" >/dev/null
wait_bg
as a sync >/dev/null
as b sync >/dev/null
as a sync >/dev/null
for m in a b; do
	out=$(as $m ls)
	expect "rerun QC v2 #adni #fromb" "$out" "$m merged concurrent title+tag edit"
	expect "from a" "$out" "$m sees from a"
	expect "from b" "$out" "$m sees from b"
	[[ -z $(git -C "$DEV/$m/data" status --porcelain) ]] || fail "$m working tree dirty"
	[[ ! -d $DEV/$m/data/.git/rebase-merge && ! -d $DEV/$m/data/.git/rebase-apply ]] || fail "$m stuck in rebase"
done
pass "concurrent edits on a and b merge without conflicts"

# --- ref resolution: ULID prefix and fuzzy title
as a doing "${id:0:10}" >/dev/null
as a add "rerun pipeline" >/dev/null
if as a done rerun </dev/null 2>"$DEV/err"; then fail "ambiguous ref accepted"; fi
expect "ambiguous" "$(cat "$DEV/err")" "ambiguous error"
as a done pipeline >/dev/null
expect "[x]" "$(as a ls | grep 'rerun pipeline')" "fuzzy done"
pass "ULID prefix resolves, ambiguous title errors, fuzzy title resolves"

# --- offline: commands succeed locally, unpushed grows, sync pushes after restore
wait_bg
git -C "$DEV/a/data" remote set-url origin "$DEV/unreachable.git"
as a add "offline one" >/dev/null || fail "offline add failed"
as a add "offline two" >/dev/null || fail "offline add failed"
if out=$(as a sync 2>/dev/null); then fail "sync succeeded while offline"; fi
n=$(grep -oE '^[0-9]+ unpushed' <<<"$out" | awk '{print $1}')
[[ ${n:-0} -ge 2 ]] || fail "unpushed count: got '$out'"
git -C "$DEV/a/data" remote set-url origin "$DEV/remote.git"
out=$(as a sync)
expect "synced" "$out" "sync after restore"
expect "offline two" "$(as b ls)" "b sees offline work"
pass "offline: local success, $n unpushed, sync pushes after restore"

# --- projects + export
as a project add "ADNI QC" >/dev/null
as a add "fetch scans" -p adni >/dev/null
as a block fetch scans -r "waiting on IT" >/dev/null
as a add "write report #adni" -p adni >/dev/null
as a doing write report >/dev/null
as a add "review pipeline" -p adni >/dev/null
as a add "groceries" -c personal >/dev/null
as a export --out "$DEV/f.md" 2>/dev/null
md=$(cat "$DEV/f.md")
expect "# Work update:" "$md" "export header"
expect "## ADNI QC" "$md" "project heading"
expect $'**Blocked**\n- fetch scans — waiting on IT' "$md" "blocked section"
expect $'**In progress**\n- write report' "$md" "in progress section"
expect $'**Next**\n- review pipeline' "$md" "next section"
expect $'**Done**\n- rerun pipeline' "$md" "done section"
grep -qF groceries <<<"$md" && fail "personal task exported"
grep -qF "from b" <<<"$md" && fail "personal (b) task exported"
md=$(as a export --tag adni)
expect "write report" "$md" "tag filter keeps tagged"
grep -qF "fetch scans" <<<"$md" && fail "tag filter kept untagged"
pass "export --out: work only, project → Done/In progress/Blocked/Next, --tag filters"

# --- archive drops open tasks
as a project archive adni >/dev/null
out=$(as a ls --history)
expect "[-] fetch scans" "$out" "archived project's task dropped"
pass "archiving a project drops its open tasks"

wait_bg
echo "all e2e checks passed"
