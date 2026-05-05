#!/usr/bin/env bash
# Go-flavored bluff patterns. Sourced by bluff-scanner.sh.
# Each pattern emits "<relative path>:<line>:BLUFF-G-NNN:<context>"
#
# Skip-marker convention (any of the three forms suppresses a hit when on
# the same line, the immediately preceding line, or anywhere inside the
# function body for body-level patterns like BLUFF-G-003):
#   //  SKIP-OK: #<ticket>           -- repository convention (preferred)
#   //  ANTI-BLUFF-EXEMPT: <reason>  -- legacy/synonym (forward-compat)
#   //  bluff-scan: no-assert-ok     -- legacy/synonym (forward-compat)

scan_go() {
  local relpath="$1" fpath="$2"

  # BLUFF-G-001: t.Skip() / t.Skipf() without an exempt marker.
  awk -v rel="${relpath}" '
    {
      cur_exempt = (index($0, "SKIP-OK:") > 0 || index($0, "ANTI-BLUFF-EXEMPT:") > 0 || index($0, "bluff-scan:") > 0)
      this_line_exempt = (NR in exempt_next) || cur_exempt
      if (cur_exempt) exempt_next[NR+1] = 1
      if ($0 ~ /[[:space:]]t\.Skip[f]?\(/ || $0 ~ /^t\.Skip[f]?\(/) {
        if (!this_line_exempt) {
          print rel ":" NR ":BLUFF-G-001:t.Skip without exempt comment"
        }
      }
    }
  ' "$fpath"

  # BLUFF-G-005: t.Run("", func(t *testing.T) { }) — empty named subtest.
  awk -v rel="${relpath}" '
    /t\.Run\("",[[:space:]]*func\(t \*testing\.T\)[[:space:]]*\{[[:space:]]*\}\)/ {
      print rel ":" NR ":BLUFF-G-005:empty t.Run subtest"
    }
  ' "$fpath"

  # BLUFF-G-006: empty TestXxx body on a single line.
  awk -v rel="${relpath}" '
    /^func Test[A-Z][A-Za-z0-9_]*\(t \*testing\.T\)[[:space:]]*\{[[:space:]]*\}[[:space:]]*$/ {
      print rel ":" NR ":BLUFF-G-006:empty test body"
    }
  ' "$fpath"

  # BLUFF-G-007: assert.True(t, true) / assert.NotNil(t, x) as a literal line.
  awk -v rel="${relpath}" '
    /^[[:space:]]*assert\.True\(t,[[:space:]]*true\)/ {
      if (index($0, "SKIP-OK:") == 0 && index($0, "ANTI-BLUFF-EXEMPT:") == 0 &&
          index($0, "bluff-scan:") == 0 &&
          (prev !~ /SKIP-OK:|ANTI-BLUFF-EXEMPT:|bluff-scan:/)) {
        print rel ":" NR ":BLUFF-G-007:assert.True(t, true) trivial"
      }
    }
    { prev = $0 }
  ' "$fpath"

  # BLUFF-G-003: test function whose body has only t.Log calls.
  #
  # Precision: pattern matching is done on the line with string and rune
  # literals + line comments stripped, so `t.Logf("describes assert.foo")`
  # doesn't trick `assert\.` matcher. Brace counting also done after
  # stripping so `}` inside string literals doesn't break body detection.
  # Body-level exempt: any of the three exempt markers anywhere in the
  # function body suppresses the hit (matches existing convention where
  # the exempt comment is placed on the line above the smoke-test logic).
  awk -v rel="${relpath}" '
    function strip_literals(line,  out, c, i, in_str, in_raw, esc) {
      out = ""; in_str = 0; in_raw = 0; esc = 0
      for (i = 1; i <= length(line); i++) {
        c = substr(line, i, 1)
        if (in_str) {
          if (esc) { esc = 0; continue }
          if (c == "\\") { esc = 1; continue }
          if (c == "\"") { in_str = 0; out = out c; continue }
          continue
        }
        if (in_raw) {
          if (c == "`") { in_raw = 0; out = out c; continue }
          continue
        }
        if (c == "\"") { in_str = 1; out = out c; continue }
        if (c == "`")  { in_raw = 1; out = out c; continue }
        if (c == "/" && substr(line, i, 2) == "//") { break }
        out = out c
      }
      return out
    }
    function flush(start_line) {
      if (start_line > 0 && asserts == 0 && logs > 0 && body_exempt == 0) {
        print rel ":" start_line ":BLUFF-G-003:no-assert test (only t.Log)"
      }
    }
    { stripped = strip_literals($0) }
    stripped ~ /^func Test[A-Z][A-Za-z0-9_]*\(t \*testing\.T\)[[:space:]]*\{/ {
      flush(start_line)
      start_line = NR; brace = 1; asserts = 0; logs = 0; body_exempt = 0
      next
    }
    start_line > 0 {
      n = gsub(/\{/, "&", stripped); brace += n
      n = gsub(/\}/, "&", stripped); brace -= n
      if (stripped ~ /t\.(Fatal|Fatalf|Error|Errorf|Skip|Skipf|FailNow|Fail|Cleanup)\(|assert\.|require\./)  asserts++
      if (stripped ~ /t\.Log[f]?\(/) logs++
      if ($0 ~ /SKIP-OK:|ANTI-BLUFF-EXEMPT:|bluff-scan:/) body_exempt = 1
      if (brace == 0) { flush(start_line); start_line = 0 }
    }
    END { flush(start_line) }
  ' "$fpath"
}
