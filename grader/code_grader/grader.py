from __future__ import annotations

import subprocess
import tempfile
from pathlib import Path
from typing import Any


def grade(task: dict[str, Any], output_files: list[dict[str, Any]]) -> dict[str, Any]:
    with tempfile.TemporaryDirectory(prefix="frameval-grade-") as temp_dir:
        root = Path(temp_dir)
        for file in output_files:
            target = root / file["path"]
            target.parent.mkdir(parents=True, exist_ok=True)
            content = file.get("content", b"")
            if isinstance(content, bytes):
                target.write_bytes(content)
            else:
                target.write_text(str(content))

        test_results: list[dict[str, Any]] = []
        passed = 0
        failed = 0
        for test_case in task.get("test_cases", []):
            proc = subprocess.run(
                test_case["command"],
                cwd=root,
                shell=True,
                capture_output=True,
                text=True,
                timeout=120,
            )
            ok = proc.returncode == 0
            if ok:
                passed += 1
            else:
                failed += 1
            test_results.append({"name": test_case["name"], "passed": ok, "output": (proc.stdout + proc.stderr).strip()})

        total = max(passed + failed, 1)
        lint_score = 10.0
        type_check_pass = True
        file_state_valid = bool(output_files)
        for file in output_files:
            text = file.get("content", "")
            if isinstance(text, bytes):
                text = text.decode("utf-8", errors="ignore")
            if "TODO" in text or "FIXME" in text:
                lint_score = min(lint_score, 7.0)
            if "any" in text and file["path"].endswith((".ts", ".tsx")):
                type_check_pass = False

        return {
            "test_pass_rate": round(passed / total, 4),
            "test_pass_count": passed,
            "test_fail_count": failed,
            "lint_score": lint_score,
            "type_check_pass": type_check_pass,
            "file_state_valid": file_state_valid,
            "test_results": test_results,
        }
