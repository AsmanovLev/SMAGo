defmodule DcTest.MixProject do
  use Mix.Project

  def project do
    [
      app: :dc_test,
      version: "0.1.0",
      elixir: "~> 1.20",
      start_permanent: Mix.env() == :prod,
      deps: deps()
    ]
  end

  def application do
    [extra_applications: [:logger], mod: {DcTest, []}]
  end

  defp deps do
    [{:jason, "~> 1.4"}]
  end
end
