import sys
import json
import subprocess
import time

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
    # Start the server
    process = subprocess.Popen(
        ["./bin/zk", "lsp"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=sys.stderr,
        cwd="/home/escoubas/zettelkasten" # root dir
    )

    # 1. Initialize
    init_req = make_request(1, "initialize", {"rootUri": "file:///home/escoubas/zettelkasten"})
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
            "uri": "file:///home/escoubas/zettelkasten/permanent_notes/test.md",
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
            "uri": "file:///home/escoubas/zettelkasten/permanent_notes/test.md"
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
