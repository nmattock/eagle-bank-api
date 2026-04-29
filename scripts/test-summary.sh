#!/usr/bin/env bash
set -euo pipefail

args=("$@")
if [ ${#args[@]} -eq 0 ]; then
  args=("./...")
fi

json_output=$(go test -json "${args[@]}" 2>&1) || true

json_lines=$(echo "$json_output" | grep -E '^\s*\{')

summary_lines=$(echo "$json_lines" | jq -s -r '
  [ .[] | select(.Test != null and (.Action == "pass" or .Action == "fail")) ] as $tests |
  [ .[] | select(.Test == null and .Action == "pass" and .Elapsed != null) ] as $pkg_times |
  ($tests | map(select(.Action == "pass")) | length) as $total_passed |
  ($tests | map(select(.Action == "fail")) | length) as $total_failed |
  ($total_passed + $total_failed) as $total |
  [ $tests[] | select(.Action == "fail") | "\(.Package)\t\(.Test)" ] as $fail_pairs |
  ($pkg_times | map({ ((.Package | sub("^eagle-bank-api/";""))): .Elapsed }) | add // {}) as $elapsed |
  ($tests | group_by(.Package) | map({
    pkg: (.[0].Package | sub("^eagle-bank-api/";"")),
    passed: [ .[] | select(.Action == "pass") ] | length,
    failed: [ .[] | select(.Action == "fail") ] | length
  })) as $by_pkg |
  [
    "TOTAL\t\($total_passed)\t\($total_failed)\t\($total)",
    ($by_pkg[] | "PKG\t\(.pkg)\t\(.passed)\t\(.failed)\t\($elapsed[.pkg] // "?")"),
    ($fail_pairs[] | "FAIL\t" + .)
  ] | .[]
')

declare -A file_all_counts
declare -A file_passed_counts

escape_regex() {
  printf '%s' "$1" | sed 's/[][(){}.^$*+?|\\]/\\&/g'
}

while IFS=$'\t' read -r kind c1 c2 c3 c4; do
  if [ "$kind" = "TEST" ]; then
    :
  fi
done <<< ""

# Build file-level counts including subtests (full test events).
all_tests=$(echo "$json_lines" | jq -s -r '
  .[] | select(.Test != null and (.Action == "pass" or .Action == "fail")) |
  [.Package, .Test, .Action] | @tsv
' | sort -u)

while IFS=$'\t' read -r pkg test_name action; do
  [ -z "$pkg" ] && continue
  [ -z "$test_name" ] && continue

  pkg_dir="${pkg#eagle-bank-api/}"
  if [ "$pkg_dir" = "$pkg" ]; then
    pkg_dir="."
  fi

  top_test_name="${test_name%%/*}"
  test_regex="$(escape_regex "$top_test_name")"
  match_line=$(grep -R -n -E "^func[[:space:]]+${test_regex}\\(" --include="*_test.go" "$pkg_dir" | awk 'NR==1{print; exit}')
  file_path="${match_line%%:*}"
  if [ -z "$file_path" ]; then
    file_path="${pkg_dir}/unknown_test_file"
  fi

  file_all_counts[$file_path]=$(( ${file_all_counts[$file_path]:-0} + 1 ))
  if [ "$action" = "pass" ]; then
    file_passed_counts[$file_path]=$(( ${file_passed_counts[$file_path]:-0} + 1 ))
  fi
done <<< "$all_tests"

echo
printf "\033[1mTest Results\033[0m\n"
echo "─────────────────────────────────────────────"

while IFS=$'\t' read -r kind c1 c2 c3 c4; do
  if [ "$kind" = "PKG" ]; then
    pkg="$c1"; passed="$c2"; failed="$c3"; elapsed="$c4"
    if [ "$failed" -gt 0 ]; then
      printf "  \033[31m✗\033[0m %-40s \033[32m%s passed\033[0m  \033[31m%s failed\033[0m  \033[2m(%ss)\033[0m\n" "$pkg" "$passed" "$failed" "$elapsed"
    else
      printf "  \033[32m✓\033[0m %-40s \033[32m%s passed\033[0m  \033[2m(%ss)\033[0m\n" "$pkg" "$passed" "$elapsed"
    fi
  fi
done <<< "$summary_lines"

echo "─────────────────────────────────────────────"
echo "By test file:"
for file in $(printf '%s\n' "${!file_all_counts[@]}" | sort); do
  total="${file_all_counts[$file]:-0}"
  passed="${file_passed_counts[$file]:-0}"
  printf "  %s: %d/%d tests passed\n" "$file" "$passed" "$total"
done
echo "─────────────────────────────────────────────"

total_passed=0
total_failed=0
total=0
while IFS=$'\t' read -r kind c1 c2 c3; do
  if [ "$kind" = "TOTAL" ]; then
    total_passed="$c1"
    total_failed="$c2"
    total="$c3"
    break
  fi
done <<< "$summary_lines"

if [ "$total_failed" -gt 0 ]; then
  printf "\033[1m\033[31mFAIL: %s/%s tests passed\033[0m\n" "$total_passed" "$total"
  echo
  printf "\033[31mFailed tests:\033[0m\n"
  while IFS=$'\t' read -r kind pkg test; do
    if [ "$kind" = "FAIL" ]; then
      short_pkg="${pkg#eagle-bank-api/}"
      printf "  FAIL: %s > %s\n" "$short_pkg" "$test"
    fi
  done <<< "$summary_lines"
  exit 1
fi

printf "\033[1m\033[32mOK: %s/%s tests passed\033[0m\n" "$total_passed" "$total"
