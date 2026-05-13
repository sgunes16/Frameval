"""Reference implementation of the wordfreq CLI.

This is the task author's canonical solution. It passes all tests in tests/.
Not visible to the agent (the agent only sees workspace/). Used by:
- agentdx validate-task (post-demo) — drops it into workspace, runs eval.sh,
  expects all tests pass.
- Task author smoke check — `cp reference/wordfreq.py workspace/ && bash eval.sh`.

Implementation notes:
- Strictly `click`-based as the task requires.
- Tokenization: split on whitespace, keep [A-Za-z]+ matches per token.
- Default case-folding to lowercase; `-c` opts into case-sensitive.
"""
from __future__ import annotations

import re
import sys
from collections import Counter
from pathlib import Path

import click

TOKEN_RE = re.compile(r"[A-Za-z]+")


@click.command()
@click.argument("path", type=click.Path(exists=False, dir_okay=False))
@click.option("-k", "top_k", default=10, show_default=True, help="Number of top words to print.")
@click.option("-c", "case_sensitive", is_flag=True, default=False, help="Case-sensitive counting.")
def main(path: str, top_k: int, case_sensitive: bool) -> None:
    p = Path(path)
    if not p.exists():
        click.echo(f"error: file not found: {path}", err=True)
        sys.exit(1)
    text = p.read_text(errors="replace")
    if not case_sensitive:
        text = text.lower()
    tokens = TOKEN_RE.findall(text)
    counter = Counter(tokens)
    for word, count in counter.most_common(top_k):
        click.echo(f"{word}: {count}")


if __name__ == "__main__":
    main()
