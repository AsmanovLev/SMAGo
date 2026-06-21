"""List all chats and check for incoming messages from any contact."""
import json, os, queue, subprocess, sys, threading, time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
RUNTIME_DIR = ROOT / "_runtime" / "deltachat-db"

# Inline minimal Rpc (same as check_messages.py)
class _Future:
    __slots__ = ("result", "error", "event")
    def __init__(self):
        self.result = None; self.error = None; self.event = threading.Event()

class Rpc:
    def __init__(self, proc):
        self.proc = proc
        self.stdin = proc.stdin; self.stdout = proc.stdout
        self._id = 0; self._id_lock = threading.Lock()
        self._futures = {}; self._futs_lock = threading.Lock()
        self._events = queue.Queue(); self._stop = threading.Event()
        threading.Thread(target=self._reader, daemon=True).start()
        threading.Thread(target=self._poll, daemon=True).start()
    def _next_id(self):
        with self._id_lock: self._id += 1; return self._id
    def _send(self, method, params):
        rid = self._next_id(); fut = _Future()
        with self._futs_lock: self._futures[rid] = fut
        body = (json.dumps({"jsonrpc":"2.0","id":rid,"method":method,"params":params}) + "\n").encode()
        self.stdin.write(body); self.stdin.flush()
        return rid, fut
    def _poll(self):
        while not self._stop.is_set():
            try: self._send("get_next_event", [])
            except: return
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
                else: fut.result = r
            fut.event.set()
    def call(self, method, *args, timeout=60):
        _, fut = self._send(method, list(args))
        if not fut.event.wait(timeout=timeout): raise TimeoutError(f"{method}")
        if fut.error: raise RuntimeError(f"{method}: {fut.error}")
        return fut.result
    def stop(self):
        self._stop.set()
        try: self.proc.terminate()
        except: pass

def main():
    time.sleep(2)
    env = os.environ.copy()
    env["DC_ACCOUNTS_PATH"] = str(RUNTIME_DIR)
    proc = subprocess.Popen(
        [r"C:\Users\User\AppData\Local\Programs\Python\Python313\Scripts\deltachat-rpc-server.EXE"],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        env=env, bufsize=0)
    rpc = Rpc(proc)
    acc_id = None
    try:
        ids = rpc.call("get_all_account_ids") or []
        if not ids: die("no accounts")
        acc_id = ids[0]
        print(f"account={acc_id}")
        print(f"waiting 60s for inbox sync...")
        # long wait to catch new incoming messages
        deadline = time.time() + 60
        seen_kinds = []
        while time.time() < deadline:
            try:
                # poll IMAP explicitly by getting fresh message count
                fmc = rpc.call("get_fresh_msg_cnt", acc_id)
                if fmc and fmc > 0:
                    print(f"  fresh_msg_cnt={fmc}")
                    # break early if we have fresh ones
                    if fmc >= 1: break
            except Exception as e:
                pass
            time.sleep(5)
        print()

        # List all chat IDs (listFlags=0 for all, no query, no contact filter)
        chat_ids = rpc.call("get_chatlist_entries", acc_id, 0, None, None) or []
        print(f"chat list: {chat_ids}")
        print()
        for entry in chat_ids:
            chat = rpc.call("get_full_chat_by_id", acc_id, entry)
            if not chat: continue
            print(f"--- chat #{entry}: {chat.get('name')} (fresh={chat.get('freshMessageCounter')}) ---")
            msg_ids = rpc.call("get_message_ids", acc_id, entry, False, False) or []
            print(f"    messages: {len(msg_ids)} ids={msg_ids}")
            if msg_ids:
                msgs_map = rpc.call("get_messages", acc_id, msg_ids) or {}
                for mid in reversed(msg_ids):
                    entry_obj = msgs_map.get(str(mid)) or msgs_map.get(mid)
                    if not entry_obj: continue
                    m = entry_obj.get("message") if isinstance(entry_obj, dict) and "message" in entry_obj else entry_obj
                    if not m: continue
                    ts = m.get("timestamp", 0)
                    ts_str = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(ts)) if ts else "?"
                    txt = (m.get("text") or "").strip()
                    fid = m.get("fromId")
                    state = m.get("state", "?")
                    view = m.get("viewType", "?")
                    print(f"    [{ts_str}Z] #{mid} from={fid} state={state} view={view}")
                    if txt:
                        for line in txt.splitlines():
                            print(f"        {line}")
                    else:
                        print(f"        (no text)")
            print()
    finally:
        try:
            if acc_id is not None: rpc.call("stop_io", acc_id)
        except: pass
        rpc.stop()

def die(msg):
    print(f"FAIL: {msg}", file=sys.stderr); sys.exit(1)

if __name__ == "__main__":
    main()
