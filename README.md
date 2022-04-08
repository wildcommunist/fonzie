# Fonzie

The interchain cosmos faucet for discord.

* Supports multiple chains at once
* Disambiguates by bech32 prefix

## Building
```bash
go build .
```

## Usage

### Environment Variables

#### An example configuration

```bash
MNEMONIC='<12 or 24 word mnemonic>'
CHAINS='[{"prefix":"umee","rpc":"https://rpc.alley.umeemania-1.network.umee.cc:443"},{"prefix":"cosmos","rpc":"https://rpc.flash.gaia-umeemania-1.network.umee.cc:443"},{"prefix":"juno","rpc":"https://rpc.section.juno-umeemania-1.network.umee.cc:443"},{"prefix":"osmo","rpc":"https://rpc.wall.osmosis-umeemania-1.network.umee.cc:443"}]'
FUNDING='{"umee":"100000000uumee","cosmos":"100000000uatom","juno":"100000000ujuno","osmo":"100000000uosmo"}'
```

### Running

```bash
./fonzie
```

### Bot Commands

See [help.md](help.md)