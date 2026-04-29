#!/usr/bin/env bash
set -euo pipefail

args=("$@")
if [ ${#args[@]} -eq 0 ]; then
  args=("./...")
fi

json_output=$(go test -json "${args[@]}" 2>&1) || true

# Filter to valid JSON lines only, then process everything in a single jq pass.
# Produces the formatted summary and exits with code 1 if any test failed.
echo "$json_output" | grep '^\s*{' | jq -s -r '
  [ .[] | select(.Test != null and (.Action == "pass" or .Action == "fail")) ] as $tests |
  [ .[] | select(.Test == null and .Action == "pass" and .Elapsed != null) ] as $pkg_times |

  ($tests | group_by(.Package) | map({
    pkg: (.[0].Package | sub("^eagle-bank-api/";"") ),
    passed: [ .[] | select(.Action == "pass") ] | length,
    failed: [ .[] | select(.Action == "fail") ] | length
  })) as $by_pkg |

  ($pkg_times | map({ ((.Package | sub("^eagle-bank-api/";""))): .Elapsed }) | add // {}) as $elapsed |

  ($tests | map(select(.Action == "pass")) | length) as $total_passed |
  ($tests | map(select(.Action == "fail")) | length) as $total_failed |
  ($total_passed + $total_failed) as $total |

  [ $tests[] | select(.Action == "fail") |
    "  FAIL: \(.Package | sub("^eagle-bank-api/";"")) > \(.Test)" ] as $fail_lines |

  "\n\u001b[1mTest Results\u001b[0m",
  "─────────────────────────────────────────────",
  ($by_pkg | sort_by(.pkg)[] |
    if .failed > 0 then
      "  \u001b[31m✗\u001b[0m \(.pkg | . + " " * ([40 - length, 1] | max))  \u001b[32m\(.passed) passed\u001b[0m  \u001b[31m\(.failed) failed\u001b[0m  \u001b[2m(\($elapsed[.pkg] // "?")s)\u001b[0m"
    else
      "  \u001b[32m✓\u001b[0m \(.pkg | . + " " * ([40 - length, 1] | max))  \u001b[32m\(.passed) passed\u001b[0m  \u001b[2m(\($elapsed[.pkg] // "?")s)\u001b[0m"
    end
  ),
  "─────────────────────────────────────────────",
  (if $total_failed > 0 then
    "\u001b[1m\u001b[31mFAIL: \($total_passed)/\($total) tests passed\u001b[0m",
    "",
    "\u001b[31mFailed tests:\u001b[0m",
    $fail_lines[],
    "\n\u001b[31mEXIT 1\u001b[0m"
  else
    "\u001b[1m\u001b[32mOK: \($total_passed)/\($total) tests passed\u001b[0m"
  end),
  if $total_failed > 0 then halt_error(1) else empty end
'
