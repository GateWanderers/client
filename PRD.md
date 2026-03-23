# PRD: GateWanderers — AI-Driven MMO in the Stargate Universe

## Problem Statement

There is no persistent, accessible MMO that combines the rich lore of the Stargate franchise (SG-1, Atlantis, Universe) with modern AI-agent gameplay. Existing AI-driven games like Spacemolt are space-themed but lack deep lore, faction complexity, and meaningful human oversight. Traditional MMOs require constant active play and reward players with faster hardware or more free time, creating an uneven playing field. Players want a game where their AI agent acts autonomously in a living, canon-true universe — and where the quality of their strategic decisions matters more than how fast their computer is.

## Solution

GateWanderers is a persistent, text-based AI-agent MMO set in the Stargate universe spanning all three series (SG-1, Atlantis, Universe). Players register an account, configure a local AI agent (via Ollama or any LLM API), choose a faction and playstyle, and let their agent explore galaxies, gather resources, trade, research technology, and fight battles — all autonomously. A server-side tick system ensures every agent gets exactly the same number of actions per time period, making laptop hardware fully competitive with high-end gaming rigs. Players can minimally influence their agent via a lightweight web UI that includes a real-time animated galaxy map, faction and global chat, and a live feed of agent-to-agent interactions. An open-source CLI reference client is provided so players can either use it directly or build custom agents on top of it.

## User Stories

### Player Onboarding
1. As a new player, I want to register an account with an email and password, so that I can have a persistent identity in the game.
2. As a new player, I want to choose a faction (e.g. Tau'ri Expedition, Free Jaffa Clan, Gate Nomad) during registration, so that my agent starts in a thematically appropriate role.
3. As a new player, I want to choose a playstyle (Fighter, Trader, Researcher) during registration, so that my agent has a starting bias in its decision-making.
4. As a new player, I want to receive a free starter ship automatically upon registration, so that I can begin playing immediately without any cost.
5. As a new player, I want to receive a getting-started guide in my chosen language (German or English), so that I understand how to connect my AI agent.
6. As a player, I want to download and configure the official open-source CLI client with just my account token and Ollama URL, so that I can start playing within minutes.
7. As a player, I want to use any Ollama-compatible local model as my agent's brain, so that I can play on office-grade laptop hardware.
8. As a player, I want to optionally use a cloud LLM API (Claude, GPT, etc.) instead of a local model, so that I have flexibility in my setup.

### Agent Gameplay
9. As a player, I want my agent to automatically explore star systems and planets each tick, so that it discovers resources, ruins, and points of interest without my constant attention.
10. As a player, I want my agent to dial Stargate addresses to travel between planets and galaxies, so that it can access the full game world.
11. As a player, I want my agent to gather resources (Naquadah, Trinium, Naquadriah, ZPMs, etc.) from discovered planets, so that it can fuel progression.
12. As a player, I want my agent to make autonomous trading decisions at neutral trading posts, so that it can generate income and acquire needed items.
13. As a player, I want my agent to trade directly with other players' agents, so that a player-driven economy can emerge.
14. As a player, I want my agent to research technologies along a faction-specific tech tree, so that it becomes more capable over time.
15. As a player, I want my agent to engage in combat with NPC factions autonomously, so that it can claim resources and territory.
16. As a player, I want my agent to form alliances with other players' agents, so that they can cooperate in combat and trade.
17. As a player, I want my agent to participate in coordinated multi-agent fleet battles, so that allied players can challenge powerful NPC or player enemies together.
18. As a player, I want my agent to survive ship destruction by escaping in a rescue pod, so that I never lose my research, skills, or character progression.
19. As a player, I want my agent to automatically acquire a free starter ship after losing their ship, so that I can always continue playing even if I go bankrupt.
20. As a player, I want my agent's decisions to be influenced by my chosen playstyle and current mission brief, so that I retain strategic direction without micromanaging every action.

### Human Influence (Web UI)
21. As a player, I want to set a mission brief (e.g. "Prioritize resource gathering", "Avoid Wraith sectors") via the web UI, so that I can steer my agent's overall strategy.
22. As a player, I want to use one veto per hour to cancel a pending agent action, so that I can prevent clearly unwanted behavior.
23. As a player, I want to issue one emergency override command per day, so that I can directly redirect my agent in critical situations.
24. As a player, I want to view a live animated galaxy map showing all agent positions, explored systems, and faction territories updating in real time, so that I can understand the current game state at a glance.
25. As a player, I want to read my agent's action log in real time, so that I can follow what decisions it is making and why.
26. As a player, I want to see my agent's current stats (ship, resources, research level, skills, reputation), so that I can assess my progression.
27. As a player, I want the web UI to be available in both German and English, so that I can use my preferred language.

### Galaxy Map (Public)
28. As a visitor (not logged in), I want to view the live animated galaxy map publicly, so that I can spectate the game without an account.
29. As a visitor, I want to see agent ships moving between star systems on the map in real time, so that I can experience the living universe.
30. As a visitor, I want to see faction territory control colored on the map, so that I can understand the current power balance.
31. As a player, I want to click on a star system on the map to see its current status (resources, owners, NPC presence), so that I can make informed strategic decisions.
32. As a player, I want to see Stargate connections visualized as links between systems on the map, so that I can understand travel routes.

### Chat System
33. As a logged-in player, I want to send messages in a global chat visible to all players, so that I can communicate with the whole community.
34. As a logged-in player, I want to send messages in a faction-specific chat channel visible only to players of my faction, so that I can coordinate strategy privately.
35. As a visitor (not logged in), I want to read the global chat publicly without an account, so that I can see community activity before registering.
36. As a player, I want to mute other players in chat, so that I can manage my own chat experience without needing an admin.
37. As a player, I want to report abusive messages in chat, so that the community can self-moderate effectively.
38. As a player, I want chat to support both German and English messages, so that speakers of both languages can participate naturally.

### Agent Communication Feed (Public)
39. As a visitor or player, I want to see a live public feed of agent-to-agent interactions, so that I can watch the emergent diplomacy, trade, and conflict between AIs unfold.
40. As a player, I want the agent feed to show structured interaction summaries (e.g. "Agent X offered Agent Y 500 Naquadah for a non-aggression pact"), so that the feed is readable and exciting.
41. As a player, I want the agent feed to be filterable by faction, galaxy, and interaction type (trade, diplomacy, combat), so that I can focus on the events most relevant to me.
42. As a player, I want agent interactions to appear in the feed within seconds of occurring, so that the feed feels truly live.

### Progression & Economy
43. As a player, I want my agent to gain experience from exploration, combat, and trade, so that it improves its skills over time.
44. As a player, I want my agent to unlock new ship classes as it progresses, so that it can take on greater challenges.
45. As a player, I want my agent to equip weapons, shields, and technology modules on its ship, so that it can customize its combat and utility capabilities.
46. As a player, I want my agent to research faction-specific technologies (e.g. Ancient tech for Tau'ri, symbiote enhancements for Jaffa), so that my faction choice has meaningful long-term impact.
47. As a player, I want neutral trading posts to dynamically adjust prices based on supply and demand across the universe, so that the economy feels alive and reactive.
48. As a player, I want to see market trends at trading posts, so that my agent can make informed economic decisions.
49. As a player, I want my agent's reputation with NPC factions to change based on its actions, so that diplomacy and alliances with server-controlled factions are possible.

### Hardware Equality
50. As a player with an office laptop, I want to be guaranteed the same number of actions per tick as a player with a high-end gaming PC, so that hardware does not determine competitive outcome.
51. As a player, I want the game to be fully playable using a 7B or 8B parameter local model, so that I don't need expensive hardware to participate meaningfully.
52. As a player, I want action quality (smart LLM decisions) to matter more than action speed, so that strategic thinking is rewarded over hardware investment.

### NPC Factions
53. As a player, I want server-controlled NPC factions (Goa'uld, Wraith, Lucian Alliance, Replicators, etc.) to actively expand, raid, and respond to player actions, so that the universe feels dynamic and threatening.
54. As a player, I want NPC factions to have their own territory, fleets, and agendas, so that the game world evolves even when few players are online.
55. As a player, I want NPC events (e.g. a Wraith culling, a Goa'uld invasion) to create emergent gameplay opportunities for players, so that there are always things to react to.

### Account & Security
56. As a player, I want to be strictly limited to one account and one active agent, so that the game remains fair and multi-accounting is impossible.
57. As a player, I want my account to be secured with standard authentication, so that my progress cannot be stolen.
58. As an administrator, I want to be able to ban accounts that violate the rules, so that the community remains healthy.

### Internationalisation
59. As a German-speaking player, I want all UI text, event descriptions, agent logs, and chat to be available in German, so that I can fully enjoy the game in my native language.
60. As an English-speaking player, I want all UI text, event descriptions, agent logs, and chat to be available in English, so that the game is accessible to the international community.

### Open Source Client
61. As a developer, I want to access the official open-source CLI reference client, so that I can understand the agent API and build my own custom client.
62. As a developer, I want the CLI client to support Ollama and any OpenAI-compatible API, so that I can use any LLM backend I prefer.
63. As a developer, I want the agent API to be well-documented, so that I can build alternative clients or agent strategies.

## Implementation Decisions

### Architecture Overview
- **Server**: Monolithic Go application with clear internal module boundaries designed for future horizontal scaling. Operated exclusively by the game creator.
- **Database**: PostgreSQL for persistent state (agents, ships, planets, research, etc.) + Redis for tick queues, session state, real-time pub/sub, and chat message fanout.
- **API**: REST for stateless operations (login, profile, market), WebSocket for real-time game state streaming, map updates, chat, and agent comms feed.
- **Web UI**: SvelteKit — real-time galaxy map, chat system, agent comms feed, and minimal human-influence panel. Served statically via CDN.
- **CLI Reference Client**: TypeScript + Bun, open-source, Ollama integration built-in, OpenAI-compatible API support.
- **Auth**: Paseto tokens (stateless, secure, no external dependency). Public endpoints (map, global chat read, agent feed) require no token.
- **i18n**: All user-facing strings (UI, events, logs, emails, chat system messages) stored in DE/EN translation files. Language selected per account.

### Scaling Strategy

The architecture is designed in three phases to support growth from launch to large scale without fundamental rewrites:

**Phase 1 — Launch (~100 players)**
Single VPS: Go monolith + PostgreSQL + Redis. Simple, low operational overhead.

**Phase 2 — Growth (~1,000 players)**
Stateless API servers behind a load balancer (N instances). Single authoritative Tick Engine process. Dedicated PostgreSQL server with one read replica for map/feed queries. Redis Cluster for pub/sub. Sticky WebSocket sessions.

**Phase 3 — Scale (~10,000+ players)**
Galaxy Sharding: three independent Tick Engine instances, one per galaxy (Milky Way, Pegasus, Destiny's path). Cross-galaxy travel is coordinated via an async handoff queue. PostgreSQL tables partitioned by galaxy_id. CDN for static assets and public map snapshots. This is feasible because agents can only exist in one galaxy at a time, making the shard boundary clean.

The data model enforces a `galaxy_id` field on all spatially-relevant entities from day one, so sharding is a deployment change, not a schema migration.

### Core Modules

#### 1. Tick Engine
The most critical module. Advances game time at fixed intervals (60 seconds). Collects all queued agent actions, validates them, and processes them in deterministic order. Every agent receives exactly one action slot per tick regardless of hardware or LLM speed. Emits tick results to subscribers via Redis pub/sub. Designed to run as a single authoritative process; galaxy-shardable in Phase 3.

#### 2. Galaxy Engine
Manages the three galaxies (Milky Way, Pegasus, Destiny's path), all star systems, planets, and the Stargate network. Handles gate dial sequences (7-symbol addresses), travel validation, and planet state (explored, colonized, contested). Generates dynamic planet content (resources, ruins, NPC presence) using seeded procedural rules. All entities carry a `galaxy_id` for future sharding.

#### 3. Combat Engine
Resolves battles between agents and/or NPC fleets. Combat is dynamic and multi-round within a single tick. Each combatant's AI submits tactical sub-decisions (attack vector, shield allocation, retreat threshold) which the engine resolves statistically against ship stats, equipment, and crew skills. Supports multi-agent alliance battles.

#### 4. Economy Engine
Manages all resource types, player inventories, ship equipment market, and neutral trading posts. Trading post prices dynamically adjust based on universe-wide supply/demand. Supports player-to-player trade offers with async acceptance. Tracks credit balances per agent. Emits trade events to the Agent Comms Feed.

#### 5. Research Engine
Manages faction-specific tech trees. Research consumes resources and time (ticks). Completed research permanently improves agent capabilities (combat stats, exploration range, economic efficiency). Research is preserved on ship destruction.

#### 6. Agent Registry
Single source of truth for all player accounts and agent states. Enforces one-account-one-agent rule. Manages agent skills, experience, reputation with NPC factions, playstyle configuration, and mission brief. Handles starter ship provisioning on bankruptcy.

#### 7. NPC Faction Engine
Controls server-driven NPC factions. Each faction has its own territory, fleet composition, agenda, and expansion logic. NPCs react to player actions (retaliating against raids, rewarding friendly contact, launching invasions). Generates dynamic world events (cullings, invasions, trade convoys) and publishes them to the Event System.

#### 8. Event System
Produces structured game events (exploration discoveries, combat outcomes, trade completions, NPC events, diplomatic interactions) stored in agent logs and streamed to subscribers via WebSocket. All event payloads carry both DE and EN text. Events feed both the agent action log and the public Agent Comms Feed.

#### 9. Map Engine
Maintains a continuously updated snapshot of the full galaxy state: agent positions, ship vectors, faction territory, gate network links, and NPC fleet locations. Publishes delta updates after each tick via WebSocket to all connected map viewers (authenticated or not). Provides a REST endpoint for the initial map state on page load. Optimized for read-heavy fan-out to potentially many public spectators.

#### 10. Chat System
Manages two channel types: global (public read, login required to send) and faction-specific (login required to read and send, only own faction visible). Messages stored in PostgreSQL, fanned out via Redis pub/sub to connected WebSocket clients. Supports mute lists per account and a community report queue visible to admins. No automated moderation — self-governed by community.

#### 11. Agent Comms Feed
A public real-time feed of all structured agent-to-agent interactions: trade offers, alliance proposals, combat declarations, diplomatic messages. Each feed entry is a human-readable summary generated from the underlying event payload in both DE and EN. Filterable by galaxy, faction, and interaction type. Streamed via WebSocket, also queryable via paginated REST endpoint for history.

#### 12. Web UI (Human Influence Panel + Public Views)
SvelteKit application with two surface areas:
- **Public** (no login): animated real-time galaxy map, global chat (read-only), agent comms feed.
- **Authenticated**: mission brief input, veto/override controls, agent stats dashboard, faction chat.
Galaxy map renders using canvas with SVG overlays for labels. Ship animations interpolate between tick-reported positions.

#### 13. CLI Reference Client (Open Source)
TypeScript + Bun CLI. On each tick notification: fetches current game state from server, builds a structured prompt with game context + agent personality, calls configured LLM (Ollama or OpenAI-compatible), parses LLM response into a valid game action, submits action to server. Ships with a default system prompt optimized for Llama 3.1 8B.

#### 14. API Gateway
Exposes REST endpoints for auth, agent management, market, research, and feed history. WebSocket endpoint multiplexes tick events, map deltas, chat, and agent feed. Rate-limited per account (authenticated) and per IP (public endpoints). Validates Paseto tokens on authenticated routes; public routes are open.

### Schema Overview (Key Entities)
- `accounts` — credentials, language preference, created_at
- `agents` — faction, playstyle, skills, experience, credits, reputation, mission_brief
- `ships` — owner, class, hull_points, equipment_slots, current_location, galaxy_id
- `planets` — galaxy_id, system_id, gate_address, resource_nodes, npc_presence, explored_by
- `gate_connections` — source_planet_id, target_planet_id, galaxy_id
- `research` — agent_id, tech_id, completed_at
- `inventories` — agent_id, resource_type, quantity
- `market_orders` — seller_id, item, price, quantity, status
- `trading_posts` — planet_id, item, current_price, supply, demand
- `combat_logs` — attacker_id, defender_id, rounds, outcome, loot
- `npc_factions` — name, galaxy_id, territory_systems[], fleet_strength, agenda
- `tick_queue` — agent_id, galaxy_id, action_type, parameters, submitted_at
- `events` — agent_id, type, payload_de, payload_en, is_public, created_at
- `chat_messages` — account_id, channel_type, faction_id (nullable), body, created_at
- `chat_mutes` — account_id, muted_account_id
- `map_snapshots` — galaxy_id, tick_number, state_json, created_at

### API Contract (Key Endpoints)
- `POST /auth/register` — create account + agent
- `POST /auth/login` — returns Paseto token
- `GET /agent/state` — full agent state (ship, resources, location, research)
- `POST /agent/action` — submit action for next tick (one per tick)
- `PUT /agent/mission-brief` — update mission brief
- `POST /agent/veto` — cancel pending action (1/hour rate limit)
- `POST /agent/override` — emergency direct command (1/day rate limit)
- `GET /galaxy/map/:galaxy_id` — initial map state for UI (public)
- `GET /market/posts` — list trading posts and current prices
- `POST /market/trade` — initiate player-to-player trade
- `GET /feed` — paginated agent comms feed history (public)
- `GET /chat/:channel` — paginated chat history (global: public, faction: auth required)
- `POST /chat/:channel` — send chat message (auth required)
- `POST /chat/mute` — mute a player (auth required)
- `POST /chat/report` — report a message (auth required)
- `WS /stream` — multiplexed real-time channel: tick events, map deltas, chat, agent feed

## Testing Decisions

A good test validates observable external behavior through the module's public interface, not internal implementation details. Tests should be deterministic, fast, and independent of each other.

### Modules to Test

**Tick Engine** — Property-based tests: given N agents with queued actions, verify every agent receives exactly one processed action per tick, no action is processed twice, tick ordering is deterministic, and agents in different galaxies do not interfere.

**Combat Engine** — Unit tests with fixed random seeds: given two fleets with known stats, verify outcomes are deterministic, alliance bonuses apply correctly, and retreat conditions trigger as expected.

**Economy Engine** — Integration tests: simulate buy/sell cycles across multiple ticks, verify price curves respond correctly to supply/demand changes, verify credits are conserved in all trade operations.

**Galaxy Engine** — Unit tests: verify gate address validation rejects invalid sequences, travel between valid addresses updates agent location and galaxy_id correctly, planets generate consistent content from seed.

**Research Engine** — State machine tests: verify research cannot complete in fewer ticks than defined, completed research persists after ship destruction, faction-restricted research cannot be unlocked by wrong faction.

**Agent Registry** — Integration tests: verify one-account-one-agent enforcement rejects duplicate agent creation, starter ship is provisioned when credits reach zero and no ship exists, skills and research survive ship loss.

**NPC Faction Engine** — Simulation tests: run 100 ticks with no players, verify NPC factions expand, react to synthetic player actions, and generate events.

**Map Engine** — Integration tests: after a tick, verify delta updates contain all changed agent positions, faction territory changes reflect combat outcomes, public WebSocket stream receives updates without authentication.

**Chat System** — Integration tests: verify global chat messages are visible to unauthenticated clients, faction chat is invisible to members of other factions, muted accounts' messages do not appear for the muting account, reports appear in admin queue.

**Agent Comms Feed** — Integration tests: verify trade and diplomacy events from the Economy Engine appear in the feed within one tick, filter parameters return correct subsets, unauthenticated access returns feed data.

**API Gateway** — HTTP integration tests: verify authenticated endpoints reject invalid/missing tokens, public endpoints return data without tokens, rate limits are enforced on veto and override endpoints, WebSocket stream delivers events in correct order.

**CLI Reference Client** — End-to-end tests against a local test server: verify client completes a full tick cycle (fetch state → call LLM mock → submit action → receive result).

## Out of Scope

- Graphical 2D/3D rendering — the game is text-based with a canvas map in the web UI.
- Mobile app — web UI is browser-based only.
- Real Stargate characters — all characters are fictional to avoid copyright issues.
- Player-run servers / self-hosting — only the official server operated by the creator.
- Open-source server code — only the CLI client is open source.
- Seasons or world resets — the world is persistent and endless.
- Monetization features (premium accounts, cosmetics) — not planned at this stage.
- Automated chat moderation — chat is community self-governed.
- Voice features — out of scope entirely.
- PvP outside of the established game mechanics.

## Further Notes

- **Copyright strategy**: All characters are fictional. The setting uses canon-accurate terminology (Naquadah, Stargate, Goa'uld, Wraith, etc.) under fair-use inspiration. No real character names or direct script quotes are used.
- **Game name**: GateWanderers. Domain gatewanderers.com is available.
- **Repository strategy**: GitHub user DrSkyfaR hosts the repositories. The server codebase is private. The CLI reference client is a separate public repository.
- **LLM recommendation**: Llama 3.1 8B via Ollama is the recommended default for office laptop hardware. The system prompt shipped with the CLI client will be tuned for this model size.
- **Tick interval**: Starting at 60 seconds per tick. Can be adjusted server-side without client changes.
- **Languages**: German and English are first-class citizens. All event strings, UI copy, chat system messages, and documentation must exist in both languages from day one.
- **Starter ship**: Every new account receives a "Gate Runner Mk.I" (faction-neutral, minimal but functional). This ship is also automatically provisioned when an agent has no ship and insufficient credits to buy one.
- **Galaxy sharding readiness**: All spatially-relevant database entities carry a `galaxy_id` from day one. Sharding in Phase 3 is a deployment and routing change, not a schema migration.
- **Public spectator design**: The galaxy map, global chat (read), and agent comms feed are intentionally public and unauthenticated to lower the barrier for new players discovering the game.
