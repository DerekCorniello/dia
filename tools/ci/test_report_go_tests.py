from contextlib import redirect_stdout
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from report_go_tests import Report


class ReportTests(unittest.TestCase):
    def events(self, *events):
        report = Report()
        with redirect_stdout(io.StringIO()):
            for event in events:
                report.feed(json.dumps(event) + "\n")
        return report

    def test_failed_subtest_and_slow_packages(self):
        report = self.events(
            {"Action": "run", "Package": "pkg", "Test": "TestA/case"},
            {"Action": "fail", "Package": "pkg", "Test": "TestA/case"},
            {"Action": "fail", "Package": "pkg", "Elapsed": 2},
            {"Action": "pass", "Package": "slow", "Elapsed": 10},
        )
        summary = report.summary("Unit tests", 1)
        self.assertIn("`pkg/TestA/case`", summary)
        self.assertIn("| `pkg` | fail | 2.00 |", summary)
        self.assertLess(summary.index("| `slow`"), summary.index("| `pkg`"))
        self.assertNotIn("still running", summary)

    def test_timeout_reports_unfinished_tests(self):
        report = self.events(
            {"Action": "start", "Package": "pkg"},
            {"Action": "run", "Package": "pkg", "Test": "TestBlocked"},
            {"Action": "output", "Package": "pkg", "Output": "panic: test timed out\n"},
        )
        summary = report.summary("Unit tests", 1)
        self.assertIn("still running", summary)
        self.assertIn("`pkg/TestBlocked`", summary)
        self.assertIn("| `pkg` | incomplete |", summary)

    def test_non_json_output_is_visible(self):
        output = io.StringIO()
        with redirect_stdout(output):
            report = Report()
            report.feed("compiler error\n")
            report.feed("[]\n")
        self.assertEqual(output.getvalue(), "compiler error\n[]\n")
        self.assertIn("No package results", report.summary("Unit tests", 1))

    def test_runner_preserves_exit_code_and_logs(self):
        with tempfile.TemporaryDirectory() as directory:
            log = Path(directory) / "results.jsonl"
            step_summary = Path(directory) / "summary.md"
            script = Path(__file__).with_name("report_go_tests.py")
            result = subprocess.run(
                [
                    sys.executable,
                    str(script),
                    "--log",
                    str(log),
                    "--",
                    sys.executable,
                    "-c",
                    "print('compiler failed'); raise SystemExit(7)",
                ],
                capture_output=True,
                text=True,
                env={**os.environ, "GITHUB_STEP_SUMMARY": str(step_summary)},
                check=False,
            )
            self.assertEqual(result.returncode, 7)
            self.assertEqual(log.read_text(), "compiler failed\n")
            self.assertIn("compiler failed", result.stdout)
            self.assertIn("Command exit code: 7", step_summary.read_text())
            self.assertEqual(
                step_summary.read_text(), log.with_suffix(".md").read_text()
            )


if __name__ == "__main__":
    unittest.main()
