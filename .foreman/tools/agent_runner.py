import argparse
import base64
import subprocess
import sys
import time

import requests


def run_agent():
    parser = argparse.ArgumentParser()
    parser.add_argument("--webhook", required=True)
    parser.add_argument("--agent", required=True)
    parser.add_argument("--title", required=True)
    parser.add_argument("--cmd", required=False)
    parser.add_argument("--prompt", required=False)
    parser.add_argument("--prompt-file", required=False)
    parser.add_argument("--cwd", required=False)
    parser.add_argument("--base64", action="store_true", help="Decode prompt/cmd from base64")
    args = parser.parse_args()

    # Wait for n8n Listener
    time.sleep(5)

    if args.cmd and (args.prompt or args.prompt_file):
        print("[Runner] Error: use either --cmd or --prompt/--prompt-file")
        requests.post(
            args.webhook,
            json={"success": False, "output": "Conflicting args: --cmd with --prompt/--prompt-file"},
        )
        sys.exit(2)

    prompt_text: str | None = None

    if args.prompt_file:
        try:
            with open(args.prompt_file, "r", encoding="utf-8") as f:
                prompt_text = f.read()
        except Exception as e:
            requests.post(
                args.webhook,
                json={"success": False, "output": f"Prompt file read error: {e}"},
            )
            sys.exit(1)
    elif args.prompt is not None:
        prompt_text = args.prompt

    if args.base64:
        try:
            if prompt_text is not None:
                print("[Runner] Decoding Base64 prompt...")
                prompt_text = base64.b64decode(prompt_text).decode("utf-8")
            elif args.cmd is not None:
                print("[Runner] Decoding Base64 cmd...")
                args.cmd = base64.b64decode(args.cmd).decode("utf-8")
        except Exception as e:
            print(f"[Runner] Base64 decode failed: {e}")
            try:
                requests.post(
                    args.webhook,
                    json={"success": False, "output": f"Base64 Error: {e}"},
                )
            except Exception as e:
                print(f"Failed to hit the webhook. Error: {e}")
            sys.exit(1)

    if prompt_text is not None:
        cmd = [
            "opencode",
            "run",
            "--agent",
            args.agent,
            "--title",
            args.title,
            prompt_text,
        ]
    elif args.cmd is not None:
        cmd = ["bash", "-lc", args.cmd]
    else:
        requests.post(
            args.webhook,
            json={"success": False, "output": "Missing prompt/cmd"},
        )
        sys.exit(2)

    print(f"[Runner] Executing: {cmd!r}")
    if args.cwd:
        print(f"[Runner] CWD: {args.cwd}")

    try:
        result = subprocess.run(cmd, cwd=args.cwd, capture_output=True, text=True)

        output = result.stdout + "\n" + result.stderr

        # BASIC CONTRACT CHECK:
        # Did the process crash?
        success = result.returncode == 0

        # Did it print a CLI error?
        if "Error:" in output or "usage:" in output:
            success = False
            output += "\n[Runner Detected CLI Error]"

    except Exception as e:
        success = False
        output = str(e)

    # Callback
    try:
        requests.post(args.webhook, json={"success": success, "output": output})
    except Exception as e:
        print(f"Failed to call webhook: {e}")


if __name__ == "__main__":
    run_agent()
