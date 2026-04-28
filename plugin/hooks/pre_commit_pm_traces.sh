#!/bin/sh
# squad pre-commit PM-trace check.
# Greps the commit subject and `git diff --staged` for backlog ID
# patterns. The squad CLI emits `chore(squad): close FEAT-NNN`
# subjects when state-machine transitions touch ledger files, so
# subjects starting with that exact prefix are allowlisted. Body
# lines are not scanned — only the first line of each message
# source — so legitimate context references in commit bodies don't
# trip the gate.
#
# PreToolUse delivers its event payload via stdin as JSON, never via env vars.
# We read the whole payload and extract tool_name and tool_input.command.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

PAYLOAD=$(cat)
[ -z "$PAYLOAD" ] && exit 0

if command -v jq >/dev/null 2>&1; then
    TOOL=$(printf '%s' "$PAYLOAD" | jq -r '.tool_name // empty' 2>/dev/null)
    CMD=$(printf '%s' "$PAYLOAD" | jq -r '.tool_input.command // empty' 2>/dev/null)
else
    TOOL=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"tool_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    CMD=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
fi

[ "$TOOL" = "Bash" ] || exit 0

case "$CMD" in
    "git commit"*|*"&& git commit"*|*"; git commit"*|*"| git commit"*) ;;
    *) exit 0 ;;
esac

PATTERN='\b(BUG|FEAT|TASK|CHORE|DEBT|BET|STD|OPS)-[0-9]+\b'

# extract_first_m walks the command and prints the argument that
# follows the FIRST `-m` flag. Quoted (single or double) and
# unquoted forms are handled. Subsequent `-m` args (treated as
# body in git's multi-paragraph mode) are intentionally ignored —
# we only scan the subject.
extract_first_m() {
    awk '
    {
        line = $0
        i = index(line, "-m ")
        j = index(line, "-m\x27")
        k = index(line, "-m\"")
        first = i
        if (j > 0 && (first == 0 || j < first)) first = j
        if (k > 0 && (first == 0 || k < first)) first = k
        if (first == 0) exit
        rest = substr(line, first + 2)
        sub(/^[[:space:]]+/, "", rest)
        ch = substr(rest, 1, 1)
        if (ch == "\"") {
            inner = substr(rest, 2)
            end = index(inner, "\"")
            if (end > 0) print substr(inner, 1, end - 1)
        } else if (ch == "\x27") {
            inner = substr(rest, 2)
            end = index(inner, "\x27")
            if (end > 0) print substr(inner, 1, end - 1)
        } else {
            if (match(rest, /^[^[:space:]]+/)) print substr(rest, RSTART, RLENGTH)
        }
    }
    '
}

SUBJECT_FROM_M=$(printf '%s' "$CMD" | extract_first_m)

SUBJECT_FROM_FILE=""
MSG_FROM_FILE_PATH=$(printf '%s' "$CMD" \
    | sed -n 's/.*-F[[:space:]]\{1,\}\([^[:space:]]*\).*/\1/p' | head -n1)
if [ -n "$MSG_FROM_FILE_PATH" ] && [ -f "$MSG_FROM_FILE_PATH" ]; then
    SUBJECT_FROM_FILE=$(head -c 65536 "$MSG_FROM_FILE_PATH" 2>/dev/null | head -n1 || printf '')
fi

SUBJECT_FROM_GITDIR=""
if [ -n "$(git rev-parse --git-dir 2>/dev/null)" ]; then
    GITDIR=$(git rev-parse --git-dir)
    if [ -f "$GITDIR/COMMIT_EDITMSG" ]; then
        SUBJECT_FROM_GITDIR=$(head -c 65536 "$GITDIR/COMMIT_EDITMSG" 2>/dev/null | head -n1 || printf '')
    fi
fi

# Allowlist: subjects starting with `chore(squad):` are squad's own
# ledger-bookkeeping commits. The squad CLI emits these on auto-fold
# / auto-archive paths and they legitimately reference item IDs to
# remain greppable. Skip the entire gate (subject scan AND staged-
# diff scan) on a match — the same commits stage `.squad/items/<ID>.md`
# files whose contents would otherwise trip the diff scan.
for SUBJECT in "$SUBJECT_FROM_M" "$SUBJECT_FROM_FILE" "$SUBJECT_FROM_GITDIR"; do
    case "$SUBJECT" in
        "chore(squad):"*) exit 0 ;;
    esac
done

FOUND=""

for CANDIDATE in "$SUBJECT_FROM_M" "$SUBJECT_FROM_FILE" "$SUBJECT_FROM_GITDIR"; do
    [ -z "$CANDIDATE" ] && continue
    HIT=$(printf '%s\n' "$CANDIDATE" | grep -E "$PATTERN" || true)
    if [ -n "$HIT" ]; then
        FOUND="${FOUND}message: ${HIT}\n"
        break
    fi
done

if command -v git >/dev/null 2>&1; then
    DIFF_HIT=$(git diff --staged --no-color 2>/dev/null \
        | grep -E "^\+" | grep -Ev "^\+\+\+ " | grep -E "$PATTERN" || true)
    [ -n "$DIFF_HIT" ] && FOUND="${FOUND}staged diff:\n${DIFF_HIT}\n"
fi

if [ -n "$FOUND" ]; then
    printf 'squad: backlog ID(s) found in commit — squad enforces no-PM-traces.\n%b' "$FOUND" 1>&2
    exit 1
fi
exit 0
