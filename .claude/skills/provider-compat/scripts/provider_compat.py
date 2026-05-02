#!/usr/bin/env python3
"""
provider_compat.py — scope inference and compat_test.go scaffolding for /provider-compat.

Usage:
  provider_compat.py --providers
  provider_compat.py --scope <name>
  provider_compat.py --write-header <name>
  provider_compat.py --help
"""

import argparse
import json
import os
import re
import sys
from pathlib import Path

REGISTRY = {
    "sendgrid": {
        "package": "internal/sendgrid",
        "handler": "internal/sendgrid/sendgrid.go",
        "port": 8101,
        "spec": "https://raw.githubusercontent.com/sendgrid/sendgrid-oai/main/oai.json",
    },
    "mailtrap": {
        "package": "internal/mailtrap",
        "handler": "internal/mailtrap/mailtrap.go",
        "port": 8100,
        "spec": "https://api-docs.mailtrap.io/docs/mailtrap-api-docs/send-emails-email-sending-api",
    },
    "twilio": {
        "package": "internal/twilio",
        "handler": "internal/twilio/twilio.go",
        "port": 8200,
        "spec": "https://raw.githubusercontent.com/twilio/twilio-oai/main/spec/json/twilio_api_v2010.json",
    },
}

# HTTP status code symbol → integer
STATUS_SYMBOLS = {
    "StatusOK": 200,
    "StatusCreated": 201,
    "StatusAccepted": 202,
    "StatusNoContent": 204,
    "StatusBadRequest": 400,
    "StatusUnauthorized": 401,
    "StatusForbidden": 403,
    "StatusNotFound": 404,
    "StatusMethodNotAllowed": 405,
    "StatusUnprocessableEntity": 422,
    "StatusTooManyRequests": 429,
    "StatusInternalServerError": 500,
}


def cmd_providers():
    """Print the provider registry as JSON."""
    print(json.dumps(REGISTRY, indent=2))


def infer_scope(source: str) -> dict:
    """Parse Go source to extract routes, auth, request fields, and status codes."""

    # Routes: HandleFunc("/<path>", handler).Methods("METHOD")
    routes = []
    for m in re.finditer(
        r'HandleFunc\(\s*"([^"]+)"\s*,\s*[\w.]+\)(?:\.Methods\("([^"]+)"\))?', source
    ):
        path, methods = m.group(1), m.group(2) or "ANY"
        routes.append({"path": path, "methods": [m.strip() for m in methods.split(",")]})

    # Auth scheme: look for BearerAuth call
    auth = "none"
    if "mailadapter.BearerAuth" in source or "BearerAuth(" in source:
        auth = "Bearer token"
    elif re.search(r'r\.Header\.Get\("Authorization"\)', source):
        auth = "Authorization header (custom)"
    elif re.search(r'r\.Header\.Get\("X-Auth-Token"\)', source):
        auth = "X-Auth-Token header"

    # Request struct fields with json tags
    fields = []
    for m in re.finditer(r'(\w+)\s+\w[\w\*\[\]\.]*\s+`json:"([^,"]+)', source):
        fields.append(m.group(2))
    fields = sorted(set(fields))

    # Response status codes (symbols and literals)
    status_codes = set()
    for sym, code in STATUS_SYMBOLS.items():
        if f"http.{sym}" in source or f"Status{sym.replace('Status','')}" in source:
            status_codes.add(code)
    for m in re.finditer(r'WriteHeader\((\d{3})\)', source):
        status_codes.add(int(m.group(1)))
    for m in re.finditer(r'WriteHeader\(http\.(\w+)\)', source):
        sym = m.group(1)
        if sym in STATUS_SYMBOLS:
            status_codes.add(STATUS_SYMBOLS[sym])
    # JSONError calls also imply status codes
    for m in re.finditer(r'JSONError\(w,\s*http\.(\w+)', source):
        sym = m.group(1)
        if sym in STATUS_SYMBOLS:
            status_codes.add(STATUS_SYMBOLS[sym])
    for m in re.finditer(r'JSONError\(w,\s*(\d{3})', source):
        status_codes.add(int(m.group(1)))

    return {
        "routes": routes,
        "auth": auth,
        "request_fields": fields,
        "status_codes": sorted(status_codes),
    }


def cmd_scope(name: str, project_root: Path):
    """Infer and print claimed scope for a provider."""
    entry = REGISTRY.get(name)
    if not entry:
        print(f"error: unknown provider '{name}'", file=sys.stderr)
        print(f"available: {', '.join(REGISTRY)}", file=sys.stderr)
        sys.exit(1)

    handler = project_root / entry["handler"]
    if not handler.exists():
        print(f"error: handler not found: {handler}", file=sys.stderr)
        sys.exit(1)

    # Collect all .go files in the package (excluding test files)
    pkg_dir = project_root / entry["package"]
    sources = []
    for f in pkg_dir.glob("*.go"):
        if not f.name.endswith("_test.go"):
            sources.append(f.read_text())

    # Routes are registered in server/server.go, not the provider package itself.
    # Extract only the HandleFunc lines that reference this provider.
    server_go = project_root / "server" / "server.go"
    if server_go.exists():
        server_src = server_go.read_text()
        provider_routes = "\n".join(
            line for line in server_src.splitlines()
            if "HandleFunc" in line and f"{name}." in line
        )
        sources.append(provider_routes)

    combined = "\n".join(sources)

    scope = infer_scope(combined)

    out = {
        "provider": name,
        "package": entry["package"],
        "port": entry["port"],
        "spec_url": entry["spec"],
        "scope": scope,
    }
    print(json.dumps(out, indent=2))


def cmd_write_header(name: str) -> str:
    """Return the DO NOT EDIT header for compat_test.go."""
    lines = [
        "// Code generated by /provider-compat. DO NOT EDIT.",
        f"// Re-run: /provider-compat {name}",
        "",
        f"package {name}_test",
        "",
    ]
    print("\n".join(lines))


def find_project_root() -> Path:
    """Walk up from cwd to find the Go module root (go.mod)."""
    p = Path.cwd()
    for _ in range(10):
        if (p / "go.mod").exists():
            return p
        p = p.parent
    return Path.cwd()


def main():
    parser = argparse.ArgumentParser(
        description="provider_compat.py — scope inference for /provider-compat"
    )
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--providers", action="store_true", help="Print provider registry as JSON")
    group.add_argument("--scope", metavar="NAME", help="Infer claimed scope from Go source")
    group.add_argument("--write-header", metavar="NAME", help="Print compat_test.go file header")

    args = parser.parse_args()
    root = find_project_root()

    if args.providers:
        cmd_providers()
    elif args.scope:
        cmd_scope(args.scope, root)
    elif args.write_header:
        cmd_write_header(args.write_header)


if __name__ == "__main__":
    main()
