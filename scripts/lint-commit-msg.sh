#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <commit-message-file>" >&2
  exit 2
fi

msg_file="$1"
first_line="$(sed -n '1p' "$msg_file" | tr -d '\r')"

# Conventional commits: type(scope)!: description
conventional_re='^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([^)]+\))?!?: .+$'
# Allow Git-generated merge commit messages.
merge_re='^Merge( branch| pull request) .+$'
# Allow Git-generated revert commit messages.
git_revert_re='^Revert ".+"$'

if [[ "$first_line" =~ $conventional_re ]] || [[ "$first_line" =~ $merge_re ]] || [[ "$first_line" =~ $git_revert_re ]]; then
  exit 0
fi

cat >&2 <<'EOF'
invalid commit message.

expected format:
  <type>(optional-scope): <description>

examples:
  feat: add profile switcher
  fix(proxy): handle empty service name
  refactor(ui)!: simplify menu update flow

allowed types:
  build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test
EOF

echo >&2
echo "got: $first_line" >&2
exit 1
