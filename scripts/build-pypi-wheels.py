#!/usr/bin/env python3
"""Build platform-specific Python wheels containing the csaw Go binary.

Uses go-to-wheel as a library, working around its assumption that the
Go main package lives at the module root (csaw's is at ./cmd/csaw).
"""

import subprocess
import sys


def main() -> int:
    version = sys.argv[1] if len(sys.argv) > 1 else "0.1.1"

    cmd = [
        sys.executable,
        "-c",
        # Monkey-patch compile_go_binary to use ./cmd/csaw instead of "."
        """
import go_to_wheel, os, subprocess, sys

_orig = go_to_wheel.compile_go_binary

def _patched(go_dir, output_path, goos, goarch, go_binary="go", ldflags=None):
    env = os.environ.copy()
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    env["CGO_ENABLED"] = "0"
    ldflags_value = "-s -w"
    if ldflags:
        ldflags_value += " " + ldflags
    cmd = [go_binary, "build", f"-ldflags={ldflags_value}", "-o", output_path, "./cmd/csaw"]
    result = subprocess.run(cmd, cwd=go_dir, env=env, capture_output=True, text=True)
    if result.returncode != 0:
        raise RuntimeError(f"Go compilation failed for {goos}/{goarch}:\\n{result.stderr}")

go_to_wheel.compile_go_binary = _patched

wheels = go_to_wheel.build_wheels(
    ".",
    name="csaw",
    version=sys.argv[1],
    output_dir="dist/pypi",
    entry_point="csaw",
    set_version_var="main.version",
    description="Mount, not install. AI workspace configuration from git-backed registries.",
    requires_python=">=3.8",
    author="Nicholas Cooper",
    license_="MIT",
    url="https://github.com/NicholasCullenCooper/csaw",
    readme="README.md",
)

print(f"Built {len(wheels)} wheel(s):")
for w in wheels:
    print(f"  {w}")
""",
        version,
    ]

    return subprocess.call(cmd)


if __name__ == "__main__":
    sys.exit(main())
