# GateWanderers CLI Client

An open-source command-line client for the GateWanderers game server.
Written in TypeScript, runs with [Bun](https://bun.sh).

---

## English

### Requirements

- [Bun](https://bun.sh) >= 1.0
- A running GateWanderers server (see `server/`)

### Installation

```bash
git clone https://github.com/yourname/gatewanderers.git
cd gatewanderers/client
bun install
```

### Configuration

Copy the example config and edit it if needed:

```bash
mkdir -p ~/.gatewanderers
cp config.example.json ~/.gatewanderers/config.json
```

Edit `~/.gatewanderers/config.json`:

```json
{
  "server_url": "http://localhost:8080",
  "token": "",
  "ollama_url": "http://localhost:11434",
  "ollama_model": "llama3.1:8b"
}
```

The `token` field is written automatically after a successful `login` or `register`.

### Commands

#### Register a new account

```bash
bun run src/index.ts register
```

You will be prompted for:
- Email address
- Password (hidden input)
- Agent name
- Faction (choose from list)
- Playstyle (fighter / trader / researcher)
- Language (default: `en`)

#### Log in

```bash
bun run src/index.ts login
```

Saves the PASETO token to `~/.gatewanderers/config.json`.

#### View agent state

```bash
bun run src/index.ts state
```

Prints a formatted table of your agent and ship, followed by the full JSON response.

### npm scripts shorthand

```bash
bun run register   # same as bun run src/index.ts register
bun run login      # same as bun run src/index.ts login
bun run state      # same as bun run src/index.ts state
```

---

## Deutsch

### Voraussetzungen

- [Bun](https://bun.sh) >= 1.0
- Ein laufender GateWanderers-Server (siehe `server/`)

### Installation

```bash
git clone https://github.com/yourname/gatewanderers.git
cd gatewanderers/client
bun install
```

### Konfiguration

Beispiel-Konfiguration kopieren und bei Bedarf anpassen:

```bash
mkdir -p ~/.gatewanderers
cp config.example.json ~/.gatewanderers/config.json
```

Inhalt von `~/.gatewanderers/config.json`:

```json
{
  "server_url": "http://localhost:8080",
  "token": "",
  "ollama_url": "http://localhost:11434",
  "ollama_model": "llama3.1:8b"
}
```

Das Feld `token` wird nach einem erfolgreichen `login` oder `register` automatisch geschrieben.

### Befehle

#### Neues Konto registrieren

```bash
bun run src/index.ts register
```

Abgefragt werden:
- E-Mail-Adresse
- Passwort (versteckte Eingabe)
- Agentenname
- Fraktion (aus Liste wählen)
- Spielstil (fighter / trader / researcher)
- Sprache (Standard: `en`)

#### Anmelden

```bash
bun run src/index.ts login
```

Speichert den PASETO-Token in `~/.gatewanderers/config.json`.

#### Agentenstatus anzeigen

```bash
bun run src/index.ts state
```

Gibt eine formatierte Tabelle mit Agent und Schiff aus, gefolgt von der vollständigen JSON-Antwort.

### npm-Skript-Kurzformen

```bash
bun run register   # entspricht: bun run src/index.ts register
bun run login      # entspricht: bun run src/index.ts login
bun run state      # entspricht: bun run src/index.ts state
```
