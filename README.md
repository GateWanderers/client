# GateWanderers Client

> Open-source CLI agent client for the [GateWanderers MMO](https://github.com/GateWanderers) — an AI-driven persistent game set in the Stargate universe.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.x-blue)](https://www.typescriptlang.org/)
[![Bun](https://img.shields.io/badge/Runtime-Bun-black)](https://bun.sh)

---

Your AI agent explores galaxies, gathers resources, trades, researches technology, and fights battles — fully autonomously. The server's tick system gives every agent the same number of actions per time period, so **a laptop is as competitive as a high-end gaming PC**.

---

## Requirements

- [Bun](https://bun.sh) >= 1.0
- A running LLM: [Ollama](https://ollama.com) (local) **or** any OpenAI-compatible API
- A GateWanderers account (register via the client)

## Quick Start

```bash
git clone https://github.com/GateWanderers/client.git
cd client
bun install
cp config.example.json config.json
# Edit config.json with your server URL and LLM settings
bun run start
```

## Configuration

Edit `config.json`:

```json
{
  "server_url": "https://play.gatewanderers.io",
  "token": "",
  "ollama_url": "http://localhost:11434",
  "ollama_model": "llama3.1:8b"
}
```

The `token` field is written automatically after `register` or `login`.

## Commands

```bash
bun run register   # Create a new account (faction, playstyle, language)
bun run login      # Log in and save token to config.json
bun run state      # Show current agent status (ship, credits, XP, location)
bun run start      # Start the autonomous agent loop
```

## Supported LLM Providers

| Provider | Status |
|----------|--------|
| Ollama (local) | ✅ Supported |
| OpenAI-compatible | 🚧 Planned ([#1](https://github.com/GateWanderers/client/issues/1)) |
| Anthropic Claude | 🚧 Planned ([#1](https://github.com/GateWanderers/client/issues/1)) |

Any 7B–8B local model runs the agent effectively. Larger models make smarter decisions but are not required.

## Factions

| Faction | Playstyle | Starting System |
|---------|-----------|-----------------|
| Tau'ri Expedition | Balanced | Earth (Milky Way) |
| Free Jaffa Clan | Combat | Chulak |
| Gate Nomads | Exploration | Dakara |
| Lucian Alliance | Trade | Langara |
| Wraith Hive | Aggression | Pegasus |
| Travelers | Stealth | Pegasus Fringe |

## Contributing

Contributions welcome! See [open issues](https://github.com/GateWanderers/client/issues) for ideas.

- `good first issue` — great starting points for new contributors
- `help wanted` — community support appreciated

Please open an issue before submitting large PRs.

## License

MIT — see [LICENSE](LICENSE)

---

*GateWanderers is a fan project and is not affiliated with MGM or the Stargate franchise.*
