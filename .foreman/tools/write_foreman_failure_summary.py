#!/usr/bin/env python3

import argparse
import json

def main() -> int:
    parser = argparse.ArgumentParser(description="Write a Foreman rejection report JSON.")
    parser.add_argument("--status", required=True)
    parser.add_argument("--reason", required=True)
    parser.add_argument("--raw-json", required=True)
    args = parser.parse_args()

    try:
        payload = json.loads(args.raw_json)
    except Exception:
        payload = None

    data = (payload.get("data") if isinstance(payload, dict) else None) or {}

    out = {
        "status": args.status,
        "reason": args.reason,
        "errors": payload.get("errors") if isinstance(payload, dict) else None,
        "data": {
            "run": data.get("run"),
            "work": data.get("work"),
        },
    }

    print(json.dumps(out, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
