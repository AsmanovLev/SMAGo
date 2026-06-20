defmodule Rpc do
  @moduledoc """
  JSON-RPC over stdio for deltachat-rpc-server.

  Each line is a complete JSON message. Events are polled via
  get_next_event (server does NOT push).
  """
  use GenServer

  def start_link(port) do
    GenServer.start_link(__MODULE__, port)
  end

  def call(pid, method, args \\ []) do
    GenServer.call(pid, {:call, method, args}, :infinity)
  end

  def next_event(pid, timeout_ms) do
    GenServer.call(pid, :next_event, timeout_ms)
  end

  @impl true
  def init(port) do
    state = %{
      port: port,
      id: 0,
      pending: %{},          # id -> {caller_pid}
      event_subs: []         # FIFO queue of callers waiting for events
    }
    {:ok, state}
  end

  @impl true
  def handle_call({:call, method, args}, from, state) do
    id = state.id + 1
    body = Jason.encode!(%{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => args}) <> "\n"
    true = Port.command(state.port, body)
    state = %{state | id: id, pending: Map.put(state.pending, id, from)}
    {:noreply, state}
  end

  def handle_call(:next_event, _from, state) do
    state = %{state | event_subs: state.event_subs ++ [self()]}
    {:noreply, state}
  end

  @impl true
  def handle_info({port, {:eol, line}}, %{port: port} = state) do
    handle_line(line, state)
  end

  def handle_info({port, {:exit, code}}, %{port: port} = state) do
    {:stop, {:port_exit, code}, state}
  end

  defp handle_line("", state), do: state

  defp handle_line(line, state) do
    case Jason.decode(line) do
      {:ok, %{"id" => id, "result" => result}} when is_integer(id) ->
        state = reply_or_discard(state, id, {:ok, result})
        case result do
          %{"contextId" => _, "event" => ev} ->
            deliver_event(state, %{"contextId" => result["contextId"], "event" => ev})
          _ ->
            state
        end

      {:ok, %{"id" => id, "error" => err}} when is_integer(id) ->
        reply_or_discard(state, id, {:error, err})

      {:ok, %{"method" => "event"} = msg} ->
        deliver_event(state, msg["params"])

      _ ->
        state
    end
  end

  defp reply_or_discard(state, id, reply) do
    case Map.pop(state.pending, id) do
      {nil, pending} ->
        %{state | pending: pending}

      {from, pending} ->
        GenServer.reply(from, reply)
        %{state | pending: pending}
    end
  end

  defp deliver_event(state, event) do
    case state.event_subs do
      [] ->
        state
      [sub | rest] ->
        GenServer.reply(sub, {:event, event})
        %{state | event_subs: rest}
    end
  end
end
