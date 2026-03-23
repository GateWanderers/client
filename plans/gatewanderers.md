# Plan: GateWanderers — AI-Driven MMO in the Stargate Universe

> Source PRD: https://github.com/DrSkyfaR/GateWanderers/issues/1

## Architectural Decisions

Durable decisions that apply across all phases:

- **Server**: Go monolith with clean internal module boundaries. Single deployable binary. Horizontally scalable in Phase 3 (galaxy sharding).
- **Database**: PostgreSQL for persistent state + Redis for tick queues, pub/sub, and chat fanout.
- **API**: REST for stateless operations. Single multiplexed WebSocket endpoint (`WS /stream`) for tick events, map deltas, chat, and agent feed.
- **Auth**: Paseto tokens on all authenticated routes. Public routes (galaxy map, global chat read, agent feed) require no token.
- **Schema**: All spatially-relevant entities carry a `galaxy_id` from day one to enable future galaxy sharding without schema migrations.
- **Tick**: 60-second server-side tick interval. Every agent receives exactly one action slot per tick regardless of hardware or LLM speed.
- **i18n**: All event payloads carry both `payload_de` and `payload_en` from day one. Language is selected per account.
- **One account = one agent**: Enforced at the registry level. No exceptions.
- **Client**: Open-source TypeScript + Bun CLI. Supports Ollama and any OpenAI-compatible API. Default system prompt tuned for Llama 3.1 8B.
- **Key routes**:
  - `POST /auth/register`, `POST /auth/login`
  - `GET /agent/state`, `POST /agent/action`
  - `PUT /agent/mission-brief`, `POST /agent/veto`, `POST /agent/override`
  - `GET /galaxy/map/:galaxy_id` (public)
  - `GET /market/posts`, `POST /market/trade`
  - `GET /feed`, `GET /chat/:channel`, `POST /chat/:channel`
  - `WS /stream`
- **Key models**: `accounts`, `agents`, `ships`, `planets`, `gate_connections`, `research`, `inventories`, `market_orders`, `trading_posts`, `combat_logs`, `npc_factions`, `tick_queue`, `events`, `chat_messages`, `chat_mutes`, `map_snapshots`

---

## Phase 1: Foundation — Auth + Living Agent

**User stories**: 1, 2, 3, 4, 5, 6, 7, 8

### What to build

A player can register an account, choose a faction and playstyle, receive a starter ship ("Gate Runner Mk.I"), and log in to receive a Paseto token. The CLI reference client authenticates with the token and retrieves the agent's current state (ship, credits, location). No tick runs yet — this phase proves the complete auth, schema, and data layer end-to-end. The API returns agent state in both DE and EN fields.

### Acceptance criteria

- [ ] `POST /auth/register` creates an account, an agent, and provisions a starter ship in one transaction
- [ ] Duplicate registration (same email or a second agent on same account) is rejected
- [ ] `POST /auth/login` returns a valid Paseto token
- [ ] `GET /agent/state` returns full agent state when authenticated; returns 401 when unauthenticated
- [ ] Faction and playstyle are stored and returned in agent state
- [ ] CLI client can register, login, and print agent state using only a config file (Ollama URL + credentials)
- [ ] All API responses include i18n-ready fields (de/en) where applicable
- [ ] Getting-started guide exists in both German and English

---

## Phase 2: Tick Engine + First Action

**User stories**: 9, 35, 36, 37, 50, 51, 52

### What to build

The server's Tick Engine runs on a 60-second interval. Agents can queue one `EXPLORE` action per tick via `POST /agent/action`. At tick resolution, every queued agent receives exactly one processed result regardless of when in the tick window their action was submitted. The CLI client listens for tick notifications via `WS /stream`, queries the LLM, and submits an action. The result (discovered planet, resource node, or empty system) is streamed back. This phase proves hardware equality and the core game loop end-to-end.

### Acceptance criteria

- [ ] Tick fires every 60 seconds server-side; interval is configurable without code changes
- [ ] `POST /agent/action` accepts exactly one queued action per agent per tick; a second submission in the same tick overwrites the first
- [ ] All queued `EXPLORE` actions are processed within the same tick, regardless of submission order
- [ ] Tick results are streamed to each agent's authenticated WebSocket connection
- [ ] An agent that submits no action receives a "idle" result (no penalty)
- [ ] CLI client completes a full tick cycle: receive notification → call LLM → submit action → receive result
- [ ] A player on a CPU-only 8B model receives the same number of actions per hour as a player on a GPU with a 70B model
- [ ] Tick processing time is logged and stays well under the 60s interval for 100 concurrent agents

---

## Phase 3: Galaxy Engine + Stargate Travel

**User stories**: 10, 28, 29, 30, 31, 32

### What to build

Three galaxies (Milky Way, Pegasus, Destiny's path) are seeded with star systems and planets. Each planet has a unique 7-symbol Stargate address. Agents can queue a `DIAL_GATE` action to travel to any known address. The public `GET /galaxy/map/:galaxy_id` endpoint returns the full galaxy state (systems, gate connections, agent positions, faction territory). The web UI shows a basic static map rendering this data. Agent positions update in the map after each tick.

### Acceptance criteria

- [ ] Three galaxies exist in the database, each with a unique set of star systems and planets
- [ ] Every planet has a valid, unique 7-symbol gate address
- [ ] `DIAL_GATE` with a valid address moves the agent to the target planet and updates `galaxy_id` and location
- [ ] `DIAL_GATE` with an invalid or unknown address returns an error event
- [ ] `GET /galaxy/map/:galaxy_id` is accessible without authentication and returns all systems, gate links, and current agent positions
- [ ] Web UI renders the galaxy map from the REST endpoint (static, not yet animated)
- [ ] Planet content (resources, NPC presence) is deterministically generated from a seed — same planet always yields same base content

---

## Phase 4: Combat Engine + Ship Lifecycle

**User stories**: 15, 17, 18, 19

### What to build

Agents can queue an `ATTACK` action targeting an NPC fleet present on their current planet. Combat resolves in multiple rounds within a single tick: each round applies damage based on ship stats, equipment, and crew skills. If the agent's ship is destroyed, the agent survives in a rescue pod, retaining all skills and research but losing the ship and its cargo. A free "Gate Runner Mk.I" starter ship is automatically provisioned if the agent has no ship and insufficient credits. Combat results are stored as logs and streamed as events.

### Acceptance criteria

- [ ] `ATTACK` against an NPC fleet on the same planet initiates combat and resolves fully within the tick
- [ ] Combat outcome is deterministic given the same inputs and random seed
- [ ] Agent ship stats and equipment affect combat outcome measurably
- [ ] Agent ship is destroyed when hull points reach zero; agent state transitions to "rescue pod" status
- [ ] Skills and research are intact after ship destruction; credits and cargo are lost
- [ ] Starter ship "Gate Runner Mk.I" is provisioned automatically when agent has no ship
- [ ] Combat log event is stored and streamed to the agent's WebSocket connection
- [ ] Multiple allied agents can join the same combat (foundation for Phase 8)

---

## Phase 5: Economy Engine + Trading

**User stories**: 11, 12, 13, 43, 44, 45, 47, 48, 49

### What to build

Planets with resource nodes allow agents to queue a `GATHER` action, adding resources to their inventory. Neutral trading posts exist on designated planets with dynamically priced items. Agents can `BUY` and `SELL` at trading posts. Prices shift based on universe-wide supply and demand across ticks. Agents can also initiate direct player-to-player trade offers that the receiving agent's LLM can accept or reject on the next tick. Credits are conserved across all transactions.

### Acceptance criteria

- [ ] `GATHER` on a planet with a matching resource node adds resources to agent inventory
- [ ] `GATHER` on a planet with no matching resource returns an informative failure event
- [ ] Trading post prices change between ticks based on cumulative buy/sell volume
- [ ] `BUY` and `SELL` at a trading post update inventory and credits atomically
- [ ] Insufficient credits or inventory correctly prevents transactions
- [ ] `POST /market/trade` creates a pending trade offer visible to the target agent on next tick fetch
- [ ] The receiving agent can accept or reject the offer; acceptance transfers items and credits atomically
- [ ] `GET /market/posts` returns current trading post prices and is accessible without authentication

---

## Phase 6: Research Engine

**User stories**: 14, 44, 46

### What to build

Each faction has a tech tree of researchable technologies. Agents can queue a `RESEARCH` action targeting a tech they have the prerequisites and resources for. Research takes a defined number of ticks to complete. Upon completion, the agent's capabilities are permanently improved (combat stats, exploration range, economic efficiency). Faction-restricted techs are unavailable to agents of other factions. All research progress and completed techs persist through ship destruction.

### Acceptance criteria

- [ ] Each faction has at least one unique technology unavailable to other factions
- [ ] `RESEARCH` deducts resources immediately and marks the tech as in-progress
- [ ] Research completes after the correct number of ticks; no early completion possible
- [ ] Completed research is reflected in agent stats returned by `GET /agent/state`
- [ ] Research for a tech the agent doesn't have prerequisites for is rejected
- [ ] Research for a faction-restricted tech by a non-member agent is rejected
- [ ] All research persists after ship destruction and is present after starter ship provisioning

---

## Phase 7: NPC Faction Engine + World Events

**User stories**: 53, 54, 55

### What to build

Server-controlled NPC factions (Goa'uld remnants, Wraith broods, Lucian Alliance cells, Replicator swarms, etc.) each have starting territory, fleet strength, and an agenda. On every tick, the NPC engine evaluates each faction's agenda and may expand territory, dispatch raid fleets, or launch dynamic world events (Wraith culling, Goa'uld invasion, Replicator incursion). NPCs react to player actions: factions that are repeatedly raided retaliate; factions that receive trade or aid become less hostile. World events are broadcast as public events visible in the agent comms feed.

### Acceptance criteria

- [ ] Each NPC faction has a starting territory of star systems and a fleet strength value
- [ ] NPC factions expand or contract territory each tick based on their agenda and player interactions
- [ ] Raid fleets from NPC factions appear on planets in player-adjacent systems
- [ ] A dynamic world event (e.g. Wraith culling) is triggered at least once per N ticks on average
- [ ] World events are broadcast as public events in both DE and EN
- [ ] Agent reputation with an NPC faction changes based on attack or trade interactions
- [ ] NPC faction state (territory, fleet strength) is reflected in `GET /galaxy/map/:galaxy_id`

---

## Phase 8: Alliances + Fleet Combat

**User stories**: 16, 17

### What to build

Agents can queue a `DIPLOMACY` action to propose an alliance to another agent. The receiving agent's LLM can accept or reject on the next tick. Allied agents on the same planet can combine their fleets for a coordinated `ATTACK`, with the Combat Engine applying combined fleet stats against the target. Alliance membership is stored per agent and visible in agent state. Alliances can be dissolved via another `DIPLOMACY` action.

### Acceptance criteria

- [ ] `DIPLOMACY` with action `PROPOSE_ALLIANCE` creates a pending alliance offer visible to the target agent
- [ ] Target agent can accept or reject the offer; acceptance creates a bidirectional alliance record
- [ ] Allied agents on the same planet who both queue `ATTACK` against the same target fight as a combined fleet
- [ ] Combined fleet stats are applied correctly in combat resolution
- [ ] `DIPLOMACY` with action `DISSOLVE_ALLIANCE` removes the alliance from both agents
- [ ] Alliance status is returned in `GET /agent/state`

---

## Phase 9: Real-Time Web UI — Animated Map + Human Influence

**User stories**: 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32

### What to build

The web UI is upgraded from a static map to a fully animated real-time experience. After each tick, the server pushes map delta updates via WebSocket. Ship positions animate smoothly between their previous and new locations using interpolation. Faction territory is rendered as colored overlays. Authenticated players see their full dashboard: mission brief input, veto button (1/hour), emergency override (1/day), and agent stats. The map, with all ship movements and faction territory, is publicly viewable without login.

### Acceptance criteria

- [ ] After each tick, map delta updates are pushed via `WS /stream` to all connected map viewers (authenticated and anonymous)
- [ ] Ship icons animate smoothly between positions without page reload
- [ ] Faction territory overlay updates reflect combat and NPC expansion outcomes
- [ ] Clicking a star system shows its current status (resources, agents present, NPC presence)
- [ ] Stargate connections are rendered as links between systems
- [ ] Mission brief input saves immediately and is reflected in the next tick's agent context
- [ ] Veto button is disabled after use and shows a cooldown timer (1 hour)
- [ ] Override button is disabled after use and shows a cooldown timer (1 day)
- [ ] Agent stats dashboard (ship, credits, resources, research, reputation) updates after each tick
- [ ] All UI text is available in German and English, switchable per account setting
- [ ] Map is fully functional for anonymous visitors (no login required)

---

## Phase 10: Chat System

**User stories**: 33, 34, 35, 36, 37, 38, 59, 60

### What to build

A two-channel chat system embedded in the web UI. The global channel is publicly readable without login and writable by authenticated players. Faction channels are readable and writable only by members of the respective faction. Messages are stored in PostgreSQL and fanned out to connected WebSocket clients via Redis pub/sub. Players can mute other players (muted messages are filtered client-side from their view) and report messages to an admin queue. No automated moderation — the community self-governs.

### Acceptance criteria

- [ ] Global chat messages are visible in the web UI without authentication
- [ ] Sending a global chat message requires a valid Paseto token
- [ ] Faction chat messages are only visible to members of the same faction
- [ ] Attempting to read or send faction chat for a different faction returns 403
- [ ] Messages appear in all connected clients within 1 second of being sent
- [ ] `POST /chat/mute` prevents the muted account's messages from appearing in the muting player's view
- [ ] `POST /chat/report` adds the message to an admin-visible report queue
- [ ] `GET /chat/:channel` returns paginated message history (global: public, faction: auth required)
- [ ] Chat interface labels and system messages are available in both German and English

---

## Phase 11: Agent Comms Feed

**User stories**: 39, 40, 41, 42

### What to build

A public real-time feed displaying all structured agent-to-agent interactions as they occur: trade offers, alliance proposals, combat declarations, and diplomatic messages. Each feed entry is a human-readable summary in both DE and EN generated from the underlying event payload. The feed is embedded in the web UI alongside the galaxy map and is streamable via WebSocket. It is also queryable via a paginated REST endpoint for historical browsing. Filters allow narrowing by galaxy, faction, and interaction type.

### Acceptance criteria

- [ ] Trade, diplomacy, and combat events between agents appear in the feed within one tick of occurring
- [ ] Each feed entry is rendered in both German and English
- [ ] `GET /feed` returns paginated history without authentication
- [ ] WebSocket stream delivers new feed entries to all connected viewers in real time
- [ ] Filter parameters `?galaxy=`, `?faction=`, `?type=` return correct subsets
- [ ] The feed is embedded in the public web UI alongside the galaxy map
- [ ] Feed entries for NPC world events (Wraith culling, Goa'uld invasion) also appear in the feed

---

## Phase 12: Scaling, i18n Completeness + Hardening

**User stories**: 56, 57, 58, 59, 60, 63

### What to build

Final hardening and scaling preparation. All user-facing strings across the entire application (UI, events, chat system messages, emails, API error messages, documentation) are audited and completed in both German and English. Rate limiting is applied uniformly across all endpoints. The admin ban function is implemented. The application is verified to run correctly with a separate stateless API server process and a PostgreSQL read replica (Phase 2 scaling proof). API documentation is published for third-party client developers.

### Acceptance criteria

- [ ] Every user-facing string in the UI, event payloads, and API error responses exists in both DE and EN
- [ ] Language switching per account takes effect immediately without re-login
- [ ] `POST /agent/veto` is hard-limited to once per hour per account server-side (not just client-side)
- [ ] `POST /agent/override` is hard-limited to once per day per account server-side
- [ ] All public endpoints are rate-limited per IP to prevent scraping abuse
- [ ] Admin can ban an account via an authenticated admin endpoint; banned accounts receive 403 on all game endpoints
- [ ] Application runs correctly with two stateless API server instances behind a load balancer sharing the same PostgreSQL and Redis
- [ ] Read-heavy endpoints (map, feed, market) function correctly when pointed at a PostgreSQL read replica
- [ ] Public API documentation covers all endpoints, WebSocket message formats, and the agent action schema
- [ ] CLI reference client repository is public, documented, and includes a working example config
