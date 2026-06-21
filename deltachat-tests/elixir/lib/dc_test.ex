# DeltaChat standalone smoke-test (Elixir).
#
# Stdlib + Jason. Spawns deltachat-rpc-server, speaks newline-delimited JSON,
# polls get_next_event, demultiplexes responses by id.
#
# Run: mix deps.get && mix run --no-halt
defmodule DcTest do
  @lang "ex"
  @server "deltachat-rpc-server"
  @account_dir Path.expand("../../_runtime/deltachat-db", __DIR__)
  File.mkdir_p!(@account_dir)

  def root, do: Path.expand("../..", __DIR__)

  def start(_type, _args) do
    cfg = Path.join(root(), "common.json") |> File.read!() |> Jason.decode!()
    invite = Path.join(root(), "invite.txt") |> File.read!() |> String.trim()

    System.put_env("DC_ACCOUNTS_PATH", @account_dir)
    port = Port.open({:spawn, @server}, [:binary, :exit_status, :line, :hide])

    {:ok, rpc} = Rpc.start_link(port)
    log("server: #{@server}")
    log("db dir: #{@account_dir}")

    acc_id_holder = :ets.new(:acc, [:public])

    try do
      {:ok, ids} = Rpc.call(rpc, "get_all_account_ids")
      acc_id =
        if ids != [] do
          hd(ids)
        else
          {:ok, new_id} = Rpc.call(rpc, "add_account")
          new_id
        end
      :ets.insert(acc_id_holder, {:id, acc_id})
      log("#{if ids == [], do: "created", else: "using existing"} account #{acc_id}")

      for {k, v} <- configs(cfg) do
        {:ok, _} = Rpc.call(rpc, "set_config", [acc_id, k, v])
      end

      log("configuring (may take 30s)...")
      {:ok, _} = Rpc.call(rpc, "configure", [acc_id])
      log("configured")

      {:ok, _} = Rpc.call(rpc, "start_io", [acc_id])
      log("io started, waiting for imap...")

      ev = wait_event(rpc, "ImapConnected", acc_id, 90_000)
      log("imap connected (#{ev["msg"]})")

      {:ok, chat_id} = Rpc.call(rpc, "secure_join", [acc_id, invite])
      log("secure-join chat=#{chat_id}, waiting for key exchange...")

      :ok = wait_progress(rpc, acc_id, 60_000)
      Process.sleep(2_000)  # ponytail: tiny settle so the inviter's last ack lands

      ts = DateTime.utc_now() |> DateTime.to_iso8601()
      text = "Hello from #{@lang} test, SMAGo deltachat #{@lang} smoke test, #{ts}"

      {:ok, msg_id} =
        Enum.reduce_while(1..5, {:ok, nil}, fn attempt, _acc ->
          case Rpc.call(rpc, "send_msg", [acc_id, chat_id, %{"text" => text}]) do
            {:ok, id} ->
              {:halt, {:ok, id}}

            {:error, err} ->
              log("send attempt #{attempt} failed: #{inspect(err)}")
              if attempt == 5, do: {:halt, {:error, err}}, else: (Process.sleep(5_000); {:cont, {:error, :retry}})
          end
        end)

      log("sent msg=#{msg_id} text=#{inspect(text)}")
      log("OK")
    after
      [{:id, acc_id}] = :ets.lookup(acc_id_holder, :id)
      try do
        Rpc.call(rpc, "stop_io", [acc_id])
      catch
        _, _ -> :ok
      end
      :ets.delete(acc_id_holder)
      Port.close(port)
    end
  end

  defp wait_event(rpc, kind, acc_id, timeout_ms) do
    deadline = System.monotonic_time(:millisecond) + timeout_ms
    do_wait_event(rpc, kind, acc_id, deadline)
  end

  defp do_wait_event(rpc, kind, acc_id, deadline) do
    remaining = max(0, deadline - System.monotonic_time(:millisecond))
    case Rpc.next_event(rpc, min(remaining, 5_000)) do
      {:event, %{"contextId" => cid, "event" => ev}} when cid == acc_id ->
        if ev["kind"] == kind do
          ev
        else
          do_wait_event(rpc, kind, acc_id, deadline)
        end

      {:event, _} ->
        do_wait_event(rpc, kind, acc_id, deadline)

      :timeout ->
        if System.monotonic_time(:millisecond) < deadline do
          do_wait_event(rpc, kind, acc_id, deadline)
        else
          die("event #{kind} not seen")
        end
    end
  end

  defp wait_progress(rpc, acc_id, timeout_ms) do
    deadline = System.monotonic_time(:millisecond) + timeout_ms
    do_wait_progress(rpc, acc_id, deadline)
  end

  defp do_wait_progress(rpc, acc_id, deadline) do
    remaining = max(0, deadline - System.monotonic_time(:millisecond))
    case Rpc.next_event(rpc, min(remaining, 5_000)) do
      {:event, %{"contextId" => cid, "event" => ev}} when cid == acc_id ->
        if ev["kind"] == "SecurejoinJoinerProgress" do
          prog = ev["progress"]
          log("secure-join progress: contact=#{ev["contactId"]} progress=#{prog}")
          if prog >= 400, do: :ok, else: do_wait_progress(rpc, acc_id, deadline)
        else
          do_wait_progress(rpc, acc_id, deadline)
        end

      {:event, _} ->
        do_wait_progress(rpc, acc_id, deadline)

      :timeout ->
        if System.monotonic_time(:millisecond) < deadline do
          do_wait_progress(rpc, acc_id, deadline)
        else
          die("secure-join did not finish in time")
        end
    end
  end

  defp configs(cfg) do
    %{
      "addr" => cfg["email"],
      "mail_pw" => cfg["password"],
      "displayname" => cfg["name"],
      "configured_mail_server" => "imap.rambler.ru",
      "configured_mail_port" => "993",
      "configured_mail_user" => cfg["email"],
      "configured_mail_pw" => cfg["password"],
      "configured_send_server" => "smtp.rambler.ru",
      "configured_send_port" => "465",
      "configured_send_user" => cfg["email"],
      "configured_send_pw" => cfg["password"]
    }
  end

  defp log(msg), do: IO.puts("[#{@lang}] #{msg}")

  defp die(msg) do
    IO.puts(:stderr, "[#{@lang}] FAIL: #{msg}")
    System.halt(1)
  end
end
