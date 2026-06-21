"""Read recent messages from the chat with delt.er@bk.ru.

Connects via the same deltachat-rpc-server. Uses the existing _runtime
database which has our keypair and prior chat history.
"""
import json, os, queue, subprocess, sys, threading, time
from pathlib import Path

LANG = "rd"  # read
ROOT = Path(__file__).resolve().parent.parent
RUNTIME_DIR = ROOT / "_runtime" / "deltachat-db"
CONTACT = "delt.er@bk.ru"

# Inline minimal Rpc (copied pattern from python/test.py for clarity)
class _Future:
    __slots__ = ("result", "error", "event")
    def __init__(self):
        self.result = None; self.error = None; self.event = threading.Event()

class Rpc:
    def __init__(self, proc):
        self.proc = proc
        self.stdin = proc.stdin
        self.stdout = proc.stdout
        self._id = 0
        self._id_lock = threading.Lock()
        self._futures = {}
        self._futs_lock = threading.Lock()
        self._events = queue.Queue()
        self._stop = threading.Event()
        threading.Thread(target=self._reader, daemon=True).start()
        threading.Thread(target=self._poll, daemon=True).start()

    def _next_id(self):
        with self._id_lock:
            self._id += 1; return self._id

    def _send(self, method, params):
        rid = self._next_id()
        fut = _Future()
        with self._futs_lock: self._futures[rid] = fut
        body = (json.dumps({"jsonrpc":"2.0","id":rid,"method":method,"params":params}) + "\n").encode()
        self.stdin.write(body); self.stdin.flush()
        return rid, fut

    def _poll(self):
        while not self._stop.is_set():
            try: self._send("get_next_event", [])
            except Exception: return
            time.sleep(0.05)

    def _reader(self):
        while not self._stop.is_set():
            line = self.stdout.readline()
            if not line: return
            line = line.strip()
            if not line: continue
            try: msg = json.loads(line)
            except: continue
            if msg.get("method") == "event": self._events.put(msg); continue
            rid = msg.get("id")
            if rid is None: continue
            with self._futs_lock: fut = self._futures.pop(rid, None)
            if fut is None: continue
            if "error" in msg: fut.error = msg["error"]
            else:
                r = msg.get("result")
                if isinstance(r, dict) and "contextId" in r and "event" in r:
                    self._events.put({"jsonrpc":"2.0","method":"event","params":r})
                    fut.result = None
                else:
                    fut.result = r
            fut.event.set()

    def call(self, method, *args, timeout=60):
        _, fut = self._send(method, list(args))
        if not fut.event.wait(timeout=timeout):
            raise TimeoutError(f"{method}")
        if fut.error: raise RuntimeError(f"{method}: {fut.error}")
        return fut.result

    def next_event(self, timeout=60):
        deadline = time.time() + timeout
        while time.time() < deadline:
            try: ev = self._events.get(timeout=0.5)
            except queue.Empty: continue
            if "_fatal" in ev: raise RuntimeError(ev["_fatal"])
            return ev.get("params")
        raise TimeoutError("event poll")

    def stop(self):
        self._stop.set()
        try: self.proc.terminate()
        except: pass

def main():
    # ponytail: small grace so the prior test's server fully releases accounts.lock
    time.sleep(2)
    env = os.environ.copy()
    env["DC_ACCOUNTS_PATH"] = str(RUNTIME_DIR)
    print(f"[{LANG}] DC_ACCOUNTS_PATH={env['DC_ACCOUNTS_PATH']}")
    print(f"[{LANG}] toml exists: {(RUNTIME_DIR / 'accounts.toml').exists()}")
    proc = subprocess.Popen(
        [r"C:\Users\User\AppData\Local\Programs\Python\Python313\Scripts\deltachat-rpc-server.EXE"],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        env=env, bufsize=0,
    )
    rpc = Rpc(proc)
    acc_id = None
    try:
        ids = rpc.call("get_all_account_ids") or []
        if not ids: die("no accounts")
        acc_id = ids[0]
        print(f"[{LANG}] account={acc_id}")

        # Wait for inbox to settle (one IMAP cycle)
        print(f"[{LANG}] waiting for inbox (up to 30s)...")
        try:
            for _ in range(60):
                ev = rpc.next_event(timeout=2)
                if ev is None: continue
                inner = ev.get("event") or {}
                if inner.get("kind") in ("ImapConnected", "MsgsChanged", "IncomingMsg", "IncomingMsgBunch"):
                    print(f"[{LANG}] ev: {inner.get('kind')}")
        except TimeoutError:
            pass

        # Find contact id for delt.er@bk.ru
        contact_ids = rpc.call("get_contact_ids", acc_id, 0, None) or []
        target_contact = None
        for cid in contact_ids:
            contact = rpc.call("get_contact", acc_id, cid)
            if contact and contact.get("address") == CONTACT:
                target_contact = cid
                print(f"[{LANG}] contact={cid} addr={contact.get('address')} name={contact.get('displayName') or contact.get('name')}")
                break
        if not target_contact:
            die(f"contact {CONTACT} not found in {contact_ids}")

        # Find chat with that contact
        chat_id = rpc.call("get_chat_id_by_contact_id", acc_id, target_contact)
        print(f"[{LANG}] chat_id={chat_id}")

        # Get message IDs in chat
        msg_ids = rpc.call("get_message_ids", acc_id, chat_id, False, False) or []
        print(f"[{LANG}] found {len(msg_ids)} messages in chat")
        print()

        # Fetch messages in batches
        batch_size = 20
        last_msgs = msg_ids[-batch_size:]  # last 20 (already sorted oldest-first)
        msgs_map = rpc.call("get_messages", acc_id, last_msgs) or {}
        print(f"[{LANG}] fetched batch of {len(msgs_map)} messages")
        print()

        for mid in reversed(last_msgs):  # newest first
            entry = msgs_map.get(str(mid)) or msgs_map.get(mid)
            if not entry:
                # MessageLoadResult can be a result object
                if isinstance(msgs_map, dict):
                    entry = next((v for k, v in msgs_map.items() if str(k) == str(mid)), None)
            if not entry: continue
            m = entry.get("message") if isinstance(entry, dict) and "message" in entry else entry
            if not m: continue
            ts = m.get("timestamp", 0)
            ts_str = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(ts)) if ts else "?"
            txt = (m.get("text") or "").strip()
            sender = m.get("fromId")
            state = m.get("state", "?")
            view = m.get("viewType", "?")
            print(f"[{ts_str}Z] msg#{mid} from={sender} state={state} view={view}")
            if txt:
                for line in txt.splitlines():
                    print(f"    {line}")
            else:
                print(f"    (no text)")
            print()

    finally:
        try:
            if acc_id is not None: rpc.call("stop_io", acc_id)
        except: pass
        rpc.stop()

def die(msg):
    print(f"[{LANG}] FAIL: {msg}", file=sys.stderr); sys.exit(1)

if __name__ == "__main__":
    main()
