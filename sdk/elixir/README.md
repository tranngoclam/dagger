> **Warning** This SDK is experimental. Please do not use it for anything
> mission-critical. Possible issues include:

- Missing features
- Stability issues
- Performance issues
- Lack of polish
- Upcoming breaking changes
- Incomplete or out-of-date documentation

# Dagger

[Dagger](dagger.io) SDK for Elixir.

## Installation

Fetch from repository by:

```elixir
def deps do
  [
    {:dagger, github: "dagger/dagger"}
  ]
end
```

## Running

Let's write a code below into a script:

```elixir
client = Dagger.connect!()

client
|> Dagger.Query.container([])
|> Dagger.Container.from(address: "hexpm/elixir:1.14.4-erlang-25.3-debian-buster-20230227-slim")
|> Dagger.Container.with_exec(args: ["elixir", "--version"])
|> Dagger.Container.stdout()
|> IO.puts()
```

Then running with:

```shell
$ _EXPERIMENT_DAGGER_CLI_BIN=dagger elixir ci.exs
```

Where `ci.exs` contains Elixir script above.
