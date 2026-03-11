import os
import sys
import json
import subprocess
from pathlib import Path


def resolve_data_root():
    env_root = os.environ.get("ZK_TEST_ROOT") or os.environ.get("ZK_DATA_DIR") or os.environ.get("ZK_PATH")
    if env_root:
        return Path(env_root).expanduser().resolve()

    cwd = Path.cwd().resolve()
    for candidate in [cwd, *cwd.parents]:
        if (candidate / "zettels").exists() or (candidate / ".zk").exists():
            return candidate

    raise SystemExit("Set ZK_TEST_ROOT, ZK_DATA_DIR, or ZK_PATH to the data repo root.")

def make_request(id, method, params):
    msg = {
        "jsonrpc": "2.0",
        "id": id,
        "method": method,
        "params": params
    }
    body = json.dumps(msg)
    content = f"Content-Length: {len(body)}\r\n\r\n{body}"
    return content

def make_notification(method, params):
    msg = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params
    }
    body = json.dumps(msg)
    content = f"Content-Length: {len(body)}\r\n\r\n{body}"
    return content

def run_test():
    zk_root = resolve_data_root()
    zk_bin = os.environ.get("ZK_BIN", "zk")
    note_uri = (zk_root / "zettels" / "test.md").as_uri()

    process = subprocess.Popen(
        [zk_bin, "lsp", "--dir", str(zk_root)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=sys.stderr,
        cwd=str(zk_root),
    )

    init_req = make_request(1, "initialize", {"rootUri": zk_root.as_uri()})
    process.stdin.write(init_req.encode())
    process.stdin.flush()

    # Read response
    # Headers
    headers = {}
    while True:
        line = process.stdout.readline().decode().strip()
        if not line:
            break
        key, value = line.split(": ")
        headers[key] = value
    
    length = int(headers["Content-Length"])
    body = process.stdout.read(length).decode()
    print("Initialize Response:", body)

    # 2. Open Document
    open_notif = make_notification("textDocument/didOpen", {
        "textDocument": {
            "uri": note_uri,
            "languageId": "markdown",
            "version": 1,
            "text": "This is a test note.\nCheck [[lin"
        }
    })
    process.stdin.write(open_notif.encode())
    process.stdin.flush()

    # 3. Completion
    # Cursor at end of "[[lin" -> line 1 (0-indexed), char 13
    comp_req = make_request(2, "textDocument/completion", {
        "textDocument": {
            "uri": note_uri
        },
        "position": {
            "line": 1,
            "character": 11
        }
    })
    process.stdin.write(comp_req.encode())
    process.stdin.flush()

    # Read response
    headers = {}
    while True:
        line = process.stdout.readline().decode().strip()
        if not line:
            break
        if ": " in line:
            key, value = line.split(": ")
            headers[key] = value
    
    length = int(headers.get("Content-Length", 0))
    if length > 0:
        body = process.stdout.read(length).decode()
        print("Completion Response:", body)

    process.terminate()

if __name__ == "__main__":
    run_test()
