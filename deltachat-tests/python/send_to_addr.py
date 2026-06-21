"""Send a message to delt.er@ro.ru (no secure-join, just plain email)."""
import json, os, queue, subprocess, sys, threading, time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
RUNTIME_DIR = ROOT / "_runtime" / "deltachat-db"
TARGET = "delt.er@ro.ru"

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

        # wait for io to be up
        print("starting io...")
        try:
            rpc.call("start_io", acc_id, timeout=10)
        except Exception as e:
            print(f"start_io (already up?): {e}")

        # Look up the contact — creates if not exists
        print(f"looking up contact {TARGET}...")
        contact_id = rpc.call("lookup_contact_id_by_addr", acc_id, TARGET)
        if contact_id is None:
            die(f"could not look up contact for {TARGET}")
        print(f"contact_id={contact_id}")

        # Get or create the chat
        chat_id = rpc.call("get_chat_id_by_contact_id", acc_id, contact_id)
        if chat_id is None or chat_id == 0:
            print(f"no existing chat, creating...")
            chat_id = rpc.call("create_chat_by_contact_id", acc_id, contact_id)
        print(f"chat_id={chat_id}")

        # Send
        text = f"yo Delter (или кто там на ro.ru), this is a plain (autocrypt) test from SMAGo deltachat-tests at {time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}"
        msg_id = rpc.call("send_msg", acc_id, chat_id, {"text": text})
        print(f"sent msg={msg_id} text={text!r}")
        print("OK")
    finally:
        try:
            if acc_id is not None: rpc.call("stop_io", acc_id)
        except: pass
        rpc.stop()

def die(msg):
    print(f"FAIL: {msg}", file=sys.stderr); sys.exit(1)

if __name__ == "__main__":
    main()
