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
        cwd="/home/escoubas/zettelkasten"
    )

    # 1. Initialize
    init_req = make_request(1, "initialize", {"rootUri": "file:///home/escoubas/zettelkasten"})
    process.stdin.write(init_req.encode())
    process.stdin.flush()
    
    # Read response (skip details)
    headers = {}
    while True:
        line = process.stdout.readline().decode().strip()
        if not line: break
        if ": " in line:
            key, value = line.split(": ")
            headers[key] = value
    
    length = int(headers.get("Content-Length", 0))
    if length > 0:
        process.stdout.read(length)
        # 2. Open Document
    open_notif = make_notification("textDocument/didOpen", {
        "textDocument": {
            "uri": "file:///home/escoubas/zettelkasten/permanent_notes/test_jump.md",
            "languageId": "markdown",
            "version": 1,
            "text": "See [[linux_programming_interface_book]] for details."
        }
    })
    process.stdin.write(open_notif.encode())
    process.stdin.flush()

    # 3. Definition
    # Cursor at "linux..." -> line 0, char 10 (start of word roughly)
    def_req = make_request(2, "textDocument/definition", {
        "textDocument": {
            "uri": "file:///home/escoubas/zettelkasten/permanent_notes/test_jump.md"
        },
        "position": {
            "line": 0,
            "character": 10
        }
    })
    process.stdin.write(def_req.encode())
    process.stdin.flush()

    # Read Definition Response
    headers = {}
    while True:
        line = process.stdout.readline().decode().strip()
        if not line: break
        if ": " in line:
            key, value = line.split(": ")
            headers[key] = value
    
    length = int(headers.get("Content-Length", 0))
    if length > 0:
        body = process.stdout.read(length).decode()
        print("Definition Response:", body)

    # 4. Hover
    hover_req = make_request(3, "textDocument/hover", {
        "textDocument": {
            "uri": "file:///home/escoubas/zettelkasten/permanent_notes/test_jump.md"
        },
        "position": {
            "line": 0,
            "character": 10
        }
    })
    process.stdin.write(hover_req.encode())
    process.stdin.flush()

    # Read Hover Response
    headers = {}
    while True:
        line = process.stdout.readline().decode().strip()
        if not line: break
        if ": " in line:
            key, value = line.split(": ")
            headers[key] = value
    
    length = int(headers.get("Content-Length", 0))
    if length > 0:
        body = process.stdout.read(length).decode()
        print("Hover Response:", body)

    process.terminate()

if __name__ == "__main__":
    run_test()
