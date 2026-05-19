# Task: build a `wordfreq` CLI

You are an autonomous coding agent working in an empty workspace. Build a small Python CLI that prints the most frequent words in a text file.

## Requirements

- Single executable script or module at `wordfreq.py` runnable as `python wordfreq.py <path> --top N`.
- Argparse for CLI parsing; no third-party dependencies.
- Reads UTF-8 text from the given path, splits on whitespace + punctuation, lowercases, counts.
- Prints the top N (default 10) as `word\tcount` lines, sorted by count descending, then alphabetically.

## Style + correctness rules

- Stdlib only. No `collections.Counter` substitutes from PyPI.
- Idiomatic Python 3.11+. Type hints on public functions.
- Empty input prints nothing and exits 0.
- Missing file exits 1 with a one-line stderr message.
- The CLI must be self-contained — no imports of files that don't exist yet.

## Verification

The harness verifies behavior with a small fixture. Acceptance:

1. `python wordfreq.py fixtures/hamlet.txt --top 5` prints exactly five tab-separated lines, sorted as documented.
2. `python wordfreq.py fixtures/empty.txt` exits 0 with no output.
3. `python wordfreq.py does/not/exist.txt` exits 1 with stderr `not found: does/not/exist.txt` (or close — substring `not found` is enough).

## Common pitfalls

- **Splitting only on whitespace** so `word,` and `word` count as different words. Use `re.findall(r"[a-z']+", text.lower())` or equivalent.
- **Printing the count column first** — the spec is `word\tcount`, in that order.
- **Sorting by count only**, leaving alphabetical ties unstable across runs.
- **Hardcoding the encoding** to ASCII; the input is UTF-8.

Stop and report when the verification commands above pass.
