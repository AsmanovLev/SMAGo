# Minimal Elixir smoke for deltachat-rpc-server.
# Run: elixir scripts/smoke.exs
System.put_env("DC_ACCOUNTS_PATH", "D:/Users/User/Desktop/SMAGo/deltachat-tests/_runtime/elixir-smoke")
File.mkdir_p!(System.get_env("DC_ACCOUNTS_PATH"))

IO.puts("opening port...")
port = Port.open({:spawn, "deltachat-rpc-server"}, [:binary, :exit_status, :line])
IO.puts("port: #{inspect(port)}")

call = fn port, method, args, id ->
  body = Jason.encode!(%{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => args}) <> "\n"
  true = Port.command(port, body)
  receive do
    {^port, {:eol, line}} -> {:ok, line}
    {^port, {:data, data}} -> {:ok, data}
  after
    30_000 -> :timeout
  end
end

IO.puts("get_all_account_ids...")
{:ok, r1} = call.(port, "get_all_account_ids", [], 1)
IO.puts("got: #{r1}")
IO.puts("done")
