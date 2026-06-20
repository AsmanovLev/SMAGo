"""DeltaChat standalone smoke-test, raw JSON-RPC over stdio.

The deltachat-rpc-server protocol is newline-delimited JSON. Events are
NOT pushed — poll `get_next_event` in a loop. See ../PROTOCOL.md.

No pip deps. Stdlib only.
"""
import json
import os
import queue
import subprocess
import sys
import threading
import time
from pathlib import Path

LANG = "py"

ROOT = Path(__file__).resolve().parent.parent
RUNTIME_DIR = ROOT / "_runtime" / "deltachat-db"
RUNTIME_DIR.mkdir(parents=True, exist_ok=True)


def _find_server():
    from shutil import which
    for cand in ["deltachat-rpc-server", "deltachat-rpc-server.exe"]:
        p = which(cand)
        if p:
            return p
    scripts = Path(sys.executable).parent / "Scripts"
    for cand in ["deltachat-rpc-server.exe", "deltachat-rpc-server"]:
        p = scripts / cand
        if p.exists():
            return str(p)
    raise FileNotFoundError("deltachat-rpc-server not found in PATH")


SERVER = _find_server()


# ---------------------------------------------------------------------------
# JSON-RPC: single reader thread, id-keyed futures
# ---------------------------------------------------------------------------
class _Future:
    __slots__ = ("result", "error", "event")
    def __init__(self):
        self.result = None
        self.error = None
        self.event = threading.Event()

class Rpc:
    def __init__(self, proc):
        self.proc = proc
        self.stdin = proc.stdin
        self.stdout = proc.stdout
        self._id = 0
        self._id_lock = threading.Lock()
        self._futures = {}     # id -> _Future
        self._futs_lock = threading.Lock()
        self._events = queue.Queue()
        self._stop = threading.Event()
        self._reader = threading.Thread(target=self._reader_loop, daemon=True)
        self._reader.start()
        self._poll_thread = threading.Thread(target=self._event_poll_loop, daemon=True)
        self._poll_thread.start()

    def _next_id(self):
        with self._id_lock:
            self._id += 1
            return self._id

    def _send(self, method, params):
        rid = self._next_id()
        body = json.dumps({"jsonrpc": "2.0", "id": rid, "method": method, "params": params}) + "\n"
        fut = _Future()
        with self._futs_lock:
            self._futures[rid] = fut
        try:
            self.stdin.write(body.encode())
            self.stdin.flush()
        except Exception as e:
            with self._futs_lock:
                self._futures.pop(rid, None)
            raise RuntimeError(f"write {method}: {e!r}")
        return rid, fut

    def _event_poll_loop(self):
        """Background thread: poll get_next_event forever. Each call blocks on
        the server until an event is available (or returns null)."""
        while not self._stop.is_set():
            try:
                self._send("get_next_event", [])
            except Exception as e:
                self._events.put({"_fatal": f"poll write: {e!r}"})
                return
            time.sleep(0.05)

    def _reader_loop(self):
        """Single reader: every line is either an event-poll response (our own
        get_next_event id) or a foreign call. Events are re-enqueued, responses
        resolve their future."""
        while not self._stop.is_set():
            line = self.stdout.readline()
            if not line:
                self._events.put({"_fatal": "stdout closed"})
                return
            line = line.strip()
            if not line:
                continue
            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                continue

            if "method" in msg and msg["method"] == "event":
                # not expected from server, but just in case
                self._events.put(msg)
                continue

            rid = msg.get("id")
            if rid is None:
                continue

            with self._futs_lock:
                fut = self._futures.pop(rid, None)

            if fut is None:
                # not our call (orphan)
                continue

            if "error" in msg:
                fut.error = msg["error"]
                fut.event.set()
                continue

            result = msg.get("result")
            # If this was a get_next_event response, the result IS the event
            # payload. Re-enqueue it. Otherwise return the raw result.
            if isinstance(result, dict) and "contextId" in result and "event" in result:
                self._events.put({"jsonrpc": "2.0", "method": "event", "params": result})
                fut.result = None  # poll has no meaningful result
            else:
                fut.result = result
            fut.event.set()

    def call(self, method, *params, timeout=120):
        _, fut = self._send(method, list(params))
        if not fut.event.wait(timeout=timeout):
            # don't leave the future hanging
            with self._futs_lock:
                self._futures.pop(id(None), None)  # fut.id doesn't exist
            raise TimeoutError(f"{method} timed out after {timeout}s")
        if fut.error is not None:
            raise RuntimeError(f"{method}: {fut.error}")
        return fut.result

    def wait_event(self, kind, acc_id=None, timeout=120):
        deadline = time.time() + timeout
        while time.time() < deadline:
            try:
                ev = self._events.get(timeout=0.5)
            except queue.Empty:
                continue
            if "_fatal" in ev:
                raise RuntimeError(ev["_fatal"])
            p = ev.get("params") or {}
            inner = p.get("event") or {}
            if inner.get("kind") != kind:
                continue
            if acc_id is not None and p.get("contextId") != acc_id:
                continue
            return {"msg": inner.get("msg", ""), "data": inner, "raw": p}
        raise TimeoutError(f"event {kind} not seen in {timeout}s")

    def stop(self):
        self._stop.set()
        try:
            self.proc.terminate()
        except Exception:
            pass


# ---------------------------------------------------------------------------
def die(msg):
    print(f"[{LANG}] FAIL: {msg}", file=sys.stderr)
    sys.exit(1)

def log(msg):
    print(f"[{LANG}] {msg}")


def main():
    cfg = json.loads((ROOT / "common.json").read_text("utf-8"))
    invite = (ROOT / "invite.txt").read_text("utf-8").strip()
    email, password, name = cfg["email"], cfg["password"], cfg["name"]

    log(f"server: {SERVER}")
    log(f"db dir: {RUNTIME_DIR}")

    env = os.environ.copy()
    env["DC_ACCOUNTS_PATH"] = str(RUNTIME_DIR)
    proc = subprocess.Popen(
        [SERVER],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        env=env, bufsize=0,
    )
    rpc = Rpc(proc)
    acc_id_holder = {"id": None}

    try:
        ids = rpc.call("get_all_account_ids", timeout=10) or []
        if ids:
            acc_id = ids[0]
            log(f"using existing account {acc_id}")
        else:
            acc_id = rpc.call("add_account", timeout=10)
            log(f"created account {acc_id}")
        acc_id_holder["id"] = acc_id

        config_keys = {
            "addr": email, "mail_pw": password, "displayname": name,
            "configured_mail_server": "imap.rambler.ru", "configured_mail_port": "993",
            "configured_mail_user": email, "configured_mail_pw": password,
            "configured_send_server": "smtp.rambler.ru", "configured_send_port": "465",
            "configured_send_user": email, "configured_send_pw": password,
        }
        for k, v in config_keys.items():
            rpc.call("set_config", acc_id, k, v, timeout=10)

        log("configuring (may take 30s)...")
        rpc.call("configure", acc_id, timeout=90)
        log("configured")

        rpc.call("start_io", acc_id, timeout=10)
        log("io started, waiting for imap...")

        ev = rpc.wait_event("ImapConnected", acc_id=acc_id, timeout=90)
        log(f"imap connected ({ev['msg']})")

        chat_id = rpc.call("secure_join", acc_id, invite, timeout=30)
        log(f"secure-join chat={chat_id}, waiting for key exchange...")

        deadline = time.time() + 60
        ready = False
        while time.time() < deadline:
            try:
                ev = rpc.wait_event("SecurejoinJoinerProgress", acc_id=acc_id, timeout=5)
            except TimeoutError:
                continue
            prog = ev["data"].get("progress", 0)
            log(f"secure-join progress: contact={ev['data'].get('contactId')} progress={prog}")
            if prog >= 400:
                ready = True
                break
        if not ready:
            die("secure-join did not finish in 60s")

        time.sleep(2)  # ponytail: tiny settle

        text = f"Hello from {LANG} test, SMAGo deltachat {LANG} smoke test, {time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}"
        for attempt in range(1, 6):
            try:
                msg_id = rpc.call("send_msg", acc_id, chat_id, {"text": text}, timeout=30)
                log(f"sent msg={msg_id} text={text!r}")
                break
            except Exception as e:
                log(f"send attempt {attempt} failed: {e}")
                if attempt == 5:
                    die(f"send_msg after 5 attempts: {e}")
                time.sleep(5)

        log("OK")
    finally:
        if acc_id_holder["id"] is not None:
            try:
                rpc.call("stop_io", acc_id_holder["id"], timeout=5)
            except Exception:
                pass
        rpc.stop()


if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        die(repr(e))
