import argparse
import json
import os
from pathlib import Path
import subprocess
import sys


class Report:
    def __init__(self):
        self.packages = {}
        self.failed_tests = []
        self.running_tests = set()
        self.output = {}

    def feed(self, line):
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            print(line, end="", flush=True)
            return
        if not isinstance(event, dict) or "Action" not in event:
            print(line, end="", flush=True)
            return
        package = event.get("Package", "unknown")
        test = event.get("Test")
        action = event["Action"]
        key = (package, test)
        if action == "output":
            self.output.setdefault(package, []).append(event.get("Output", ""))
        elif action == "run" and test:
            self.running_tests.add(key)
        elif action in ("pass", "fail", "skip") and test:
            self.running_tests.discard(key)
            if action == "fail":
                self.failed_tests.append(key)
                print(f"FAIL {package}/{test}", flush=True)
        elif not test:
            elapsed = event.get("Elapsed", 0)
            self.packages[package] = (action, elapsed)
            if action == "start":
                print(f"START {package}", flush=True)
            elif action in ("pass", "fail", "skip"):
                print(f"{action.upper()} {package} ({elapsed:.2f}s)", flush=True)
                if action == "fail":
                    print("".join(self.output.get(package, [])), end="", flush=True)
                self.output.pop(package, None)

    def summary(self, label, exit_code):
        lines = [f"### {label}", "", f"Command exit code: {exit_code}", ""]
        if self.failed_tests:
            lines.extend(["Failed tests:", ""])
            lines.extend(f"- `{package}/{test}`" for package, test in self.failed_tests)
            lines.append("")
        if self.running_tests:
            lines.extend(
                [
                    "Tests still running when the command ended (check for a timeout or crash):",
                    "",
                ]
            )
            lines.extend(
                f"- `{package}/{test}`" for package, test in sorted(self.running_tests)
            )
            lines.append("")
        if self.packages:
            lines.extend(["| Package | Result | Seconds |", "| --- | --- | ---: |"])
            for package, (action, elapsed) in sorted(
                self.packages.items(), key=lambda item: item[1][1], reverse=True
            ):
                result = "incomplete" if action == "start" else action
                lines.append(f"| `{package}` | {result} | {elapsed:.2f} |")
        else:
            lines.append(
                "No package results were emitted. Check the command log for setup or compilation errors."
            )
        return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--log", required=True)
    parser.add_argument("--label", default="Go tests")
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command
    if command and command[0] == "--":
        command = command[1:]
    if not command:
        parser.error("a command is required after --")

    log_path = Path(args.log)
    log_path.parent.mkdir(parents=True, exist_ok=True)
    report = Report()
    with log_path.open("w", encoding="utf-8") as log:
        try:
            with subprocess.Popen(
                command,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                encoding="utf-8",
                errors="replace",
            ) as process:
                for line in process.stdout:
                    log.write(line)
                    log.flush()
                    report.feed(line)
                exit_code = process.wait()
        except OSError as error:
            message = f"Unable to start test command: {error}\n"
            log.write(message)
            print(message, end="", file=sys.stderr)
            exit_code = 1

    summary = report.summary(args.label, exit_code)
    log_path.with_suffix(".md").write_text(summary, encoding="utf-8")
    summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary_path:
        with open(summary_path, "a", encoding="utf-8") as destination:
            destination.write(summary)
    return exit_code if exit_code >= 0 else 1


if __name__ == "__main__":
    sys.exit(main())
