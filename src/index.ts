#!/usr/bin/env bun
/**
 * GateWanderers CLI — entry point.
 * Usage:
 *   bun run src/index.ts register
 *   bun run src/index.ts login
 *   bun run src/index.ts state
 */

import * as readline from "node:readline";
import { loadConfig, saveToken } from "./config.ts";
import { ApiClient, buildStreamURL } from "./api.ts";
import { createProvider } from "./llm.ts";
import { Dashboard } from "./dashboard.ts";
import type { DashboardState } from "./dashboard.ts";
import type { Faction, Playstyle, RegisterRequest } from "./types.ts";

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

function createReadline(): readline.Interface {
  return readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });
}

async function prompt(rl: readline.Interface, question: string): Promise<string> {
  return new Promise((resolve) => {
    rl.question(question, (answer) => resolve(answer.trim()));
  });
}

async function promptSecret(question: string): Promise<string> {
  // Write prompt without echo using raw stdin when available.
  process.stdout.write(question);
  return new Promise((resolve) => {
    const chunks: Buffer[] = [];
    process.stdin.setRawMode?.(true);
    process.stdin.resume();
    process.stdin.on("data", function handler(chunk: Buffer) {
      const byte = chunk[0];
      // Enter key
      if (byte === 13 || byte === 10) {
        process.stdin.setRawMode?.(false);
        process.stdin.pause();
        process.stdin.removeListener("data", handler);
        process.stdout.write("\n");
        resolve(Buffer.concat(chunks).toString("utf-8"));
      } else if (byte === 3) {
        // Ctrl-C
        process.stdout.write("\n");
        process.exit(1);
      } else if (byte === 127) {
        // Backspace
        if (chunks.length > 0) chunks.pop();
      } else {
        chunks.push(chunk);
      }
    });
  });
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

async function cmdRegister(): Promise<void> {
  const config = loadConfig();
  const client = new ApiClient(config.server_url);
  const rl = createReadline();

  console.log("=== GateWanderers — Register ===\n");
  const email = await prompt(rl, "Email:       ");
  rl.close();
  const password = await promptSecret("Password:    ");
  const rl2 = createReadline();
  const agentName = await prompt(rl2, "Agent name:  ");

  console.log("\nAvailable factions:");
  const factions: Faction[] = [
    "tau_ri",
    "free_jaffa",
    "gate_nomad",
    "system_lord_remnant",
    "wraith_brood",
    "ancient_seeker",
  ];
  factions.forEach((f, i) => console.log(`  ${i + 1}. ${f}`));
  const factionInput = await prompt(rl2, "Faction (name or number): ");
  const faction = resolveFaction(factionInput, factions);
  if (!faction) {
    rl2.close();
    console.error("Invalid faction.");
    process.exit(1);
  }

  console.log("\nAvailable playstyles:");
  const playstyles: Playstyle[] = ["fighter", "trader", "researcher"];
  playstyles.forEach((p, i) => console.log(`  ${i + 1}. ${p}`));
  const playstyleInput = await prompt(rl2, "Playstyle (name or number): ");
  const playstyle = resolvePlaystyle(playstyleInput, playstyles);
  if (!playstyle) {
    rl2.close();
    console.error("Invalid playstyle.");
    process.exit(1);
  }

  const language = (await prompt(rl2, "Language [en]: ")) || "en";
  rl2.close();

  const req: RegisterRequest = {
    email,
    password,
    agent_name: agentName,
    faction,
    playstyle,
    language,
  };

  try {
    console.log("\nRegistering...");
    const res = await client.register(req);
    saveToken(res.token);
    console.log("\nRegistration successful! Token saved to ~/.gatewanderers/config.json\n");
    console.log("Agent created:");
    printTable({
      Name: res.agent.name,
      Faction: res.agent.faction,
      Playstyle: res.agent.playstyle,
      Credits: String(res.agent.credits),
      Experience: String(res.agent.experience),
    });
  } catch (err: unknown) {
    console.error("\nRegistration failed:", (err as Error).message);
    process.exit(1);
  }
}

async function cmdLogin(): Promise<void> {
  const config = loadConfig();
  const client = new ApiClient(config.server_url);
  const rl = createReadline();

  console.log("=== GateWanderers — Login ===\n");
  const email = await prompt(rl, "Email:    ");
  rl.close();
  const password = await promptSecret("Password: ");

  try {
    console.log("\nLogging in...");
    const res = await client.login(email, password);
    saveToken(res.token);
    console.log("Login successful! Token saved to ~/.gatewanderers/config.json");
  } catch (err: unknown) {
    console.error("\nLogin failed:", (err as Error).message);
    process.exit(1);
  }
}

async function cmdState(): Promise<void> {
  const config = loadConfig();

  if (!config.token) {
    console.error("No token found. Please run 'login' or 'register' first.");
    process.exit(1);
  }

  const client = new ApiClient(config.server_url, config.token);

  try {
    const state = await client.getAgentState();

    console.log("\n=== Agent ===");
    printTable({
      ID: state.agent.id,
      Name: state.agent.name,
      Faction: state.agent.faction,
      Playstyle: state.agent.playstyle,
      Credits: String(state.agent.credits),
      Experience: String(state.agent.experience),
      "Mission Brief": state.agent.mission_brief || "(none)",
    });

    console.log("\n=== Ship ===");
    printTable({
      ID: state.ship.id,
      Name: state.ship.name,
      Class: state.ship.class,
      Hull: `${state.ship.hull_points} / ${state.ship.max_hull_points}`,
      Galaxy: state.ship.galaxy_id,
      System: state.ship.system_id,
      Planet: state.ship.planet_id ?? "(none)",
    });

    console.log("\n--- Full JSON ---");
    console.log(JSON.stringify(state, null, 2));
  } catch (err: unknown) {
    console.error("Failed to fetch state:", (err as Error).message);
    process.exit(1);
  }
}

// ---------------------------------------------------------------------------
// Agent loop
// ---------------------------------------------------------------------------

/**
 * Connect to the game stream and run the AI agent loop.
 * Automatically reconnects with exponential backoff on disconnect.
 * Handles SIGINT for graceful shutdown (finishes the current tick action first).
 */
async function cmdAgent(): Promise<void> {
  const config = loadConfig();

  if (!config.token) {
    console.error("No token found. Please run 'login' or 'register' first.");
    process.exit(1);
  }

  const maxRetries = config.max_retries ?? 5;
  const llm        = createProvider(config.llm!);
  const client     = new ApiClient(config.server_url, config.token, maxRetries);
  const dash       = new Dashboard();
  let retryDelay   = 1000; // ms — WS reconnect backoff, max 30s

  // Dashboard state — updated incrementally each tick.
  const ds: DashboardState = {
    tick:          0,
    agentName:     '—',
    faction:       '—',
    credits:       0,
    xp:            0,
    shipName:      '—',
    shipClass:     '—',
    hullPct:       100,
    hullCur:       100,
    hullMax:       100,
    systemID:      '—',
    galaxyID:      '—',
    planetID:      null,
    recentActions: [],
    lastDecision:  '—',
    wsConnected:   false,
  };

  function addAction(entry: string): void {
    ds.recentActions.unshift(entry);
    if (ds.recentActions.length > 5) ds.recentActions.pop();
  }

  // Graceful shutdown: set via SIGINT; finishes current tick first.
  let shutdownRequested = false;
  let activeWs: WebSocket | null = null;

  process.on("SIGINT", () => {
    if (shutdownRequested) {
      process.stdout.write("\nForce exit.\n");
      process.exit(1);
    }
    shutdownRequested = true;
    process.stdout.write("\n[SIGINT] Shutdown requested — finishing current action…\n");
    activeWs?.close();
  });

  console.log("=== GateWanderers — AI Agent ===");
  console.log(`LLM: ${llm.name} (${config.llm!.model})  |  Server: ${config.server_url}  |  Max retries: ${maxRetries}`);
  console.log("Connecting…\n");

  while (!shutdownRequested) {
    await new Promise<void>((resolve) => {
      const url = buildStreamURL(config.server_url, config.token);
      const ws  = new WebSocket(url);
      activeWs  = ws;

      ws.onopen = () => {
        retryDelay    = 1000;
        ds.wsConnected = true;
        dash.render(ds);
      };

      ws.onmessage = async (event: MessageEvent) => {
        if (shutdownRequested) return;

        let msg: Record<string, unknown>;
        try {
          msg = JSON.parse(event.data as string) as Record<string, unknown>;
        } catch {
          return;
        }

        const msgType = msg["type"] as string | undefined;

        if (msgType === "tick") {
          ds.tick = msg["tick"] as number;
          dash.render(ds);

          // Fetch state with retry.
          let agentState;
          try {
            agentState = await client.getAgentState();
          } catch (err) {
            addAction(`✗ State fetch failed: ${(err as Error).message}`);
            dash.render(ds);
            return;
          }

          const { agent, ship } = agentState;
          ds.agentName = agent.name;
          ds.faction   = agent.faction;
          ds.credits   = agent.credits;
          ds.xp        = agent.experience;
          ds.shipName  = ship.name;
          ds.shipClass = ship.class;
          ds.hullCur   = ship.hull_points;
          ds.hullMax   = ship.max_hull_points;
          ds.hullPct   = Math.round((ship.hull_points / ship.max_hull_points) * 100);
          ds.systemID  = ship.system_id;
          ds.galaxyID  = ship.galaxy_id;
          ds.planetID  = ship.planet_id;
          dash.render(ds);

          // Build LLM prompt.
          const hullPct = ds.hullPct;
          const llmPrompt = [
            "You are an AI agent in GateWanderers, a space strategy MMO in the Stargate universe.",
            "",
            "=== YOUR STATUS ===",
            `Faction: ${agent.faction} | Playstyle: ${agent.playstyle}`,
            `Credits: ${agent.credits} | XP: ${agent.experience}`,
            `Ship: ${ship.name} (${ship.class}) — Hull: ${ship.hull_points}/${ship.max_hull_points} (${hullPct}%)`,
            `Location: galaxy=${ship.galaxy_id} system=${ship.system_id}${ship.planet_id ? ` planet=${ship.planet_id}` : ""}`,
            `Mission brief: ${agent.mission_brief || "(none)"}`,
            "",
            "=== AVAILABLE ACTIONS ===",
            "EXPLORE       — scan current system, gain XP",
            "GATHER        — collect resources in current system",
            "MINE          — mine ore/minerals (requires planet)",
            "SURVEY        — survey planet for resources",
            "DIAL_GATE     — travel through stargate to another system",
            "ATTACK        — attack enemies in current system",
            "DEFEND        — defend current system, gain defense strength",
            "DIPLOMACY     — perform diplomatic action",
            "RESEARCH      — advance research tree",
            "BUY           — buy goods from market",
            "SELL          — sell goods on market",
            "ACCEPT_TRADE  — accept a pending trade offer",
            "REPAIR        — repair ship hull (costs credits)",
            "UPGRADE       — upgrade ship (costs credits)",
            "BUY_SHIP      — buy a new ship (costs credits)",
            "",
            "=== STRATEGY HINTS ===",
            hullPct < 30 ? "WARNING: Hull critically low — consider REPAIR." : "",
            agent.credits < 500 ? "Low credits — prioritize GATHER or SELL." : "",
            "",
            'Respond with ONLY a JSON object on one line: {"action": "ACTION_NAME"}',
          ].filter((l) => l !== "").join("\n");

          // Ask LLM.
          let action: string;
          try {
            action = await llm.complete(llmPrompt, agentState);
          } catch (err) {
            action = "EXPLORE";
            addAction(`✗ LLM error: ${(err as Error).message}`);
          }
          ds.lastDecision = action;
          dash.render(ds);

          // Submit action with retry.
          try {
            await client.submitAction(action);
            addAction(`✓ ${action}`);
          } catch (err) {
            addAction(`✗ ${action} (submit failed)`);
          }
          dash.render(ds);

          if (shutdownRequested) {
            process.stdout.write("[shutdown] Action complete. Disconnecting…\n");
            ws.close();
          }

        } else if (msgType === "event") {
          const eventData = msg["event"] as Record<string, unknown> | undefined;
          if (eventData) {
            const payload = (eventData["payload_en"] as string) ?? "";
            if (payload) addAction(`• ${payload.slice(0, 40)}`);
            dash.render(ds);
          }
        } else if (msgType === "connected") {
          ds.tick = msg["tick"] as number ?? ds.tick;
          dash.render(ds);
        }
      };

      ws.onerror = () => {
        ds.wsConnected = false;
        dash.render(ds);
      };

      ws.onclose = () => {
        activeWs       = null;
        ds.wsConnected = false;
        if (shutdownRequested) {
          resolve();
          return;
        }
        const jitter = Math.floor(Math.random() * 500);
        const delay  = retryDelay + jitter;
        process.stdout.write(`\nWebSocket disconnected. Reconnecting in ${(delay / 1000).toFixed(1)}s…\n`);
        retryDelay = Math.min(retryDelay * 2, 30_000);
        setTimeout(() => resolve(), delay);
      };
    });
  }

  process.stdout.write("\nAgent stopped. Goodbye.\n");
  process.exit(0);
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

function printTable(data: Record<string, string>): void {
  const keyWidth = Math.max(...Object.keys(data).map((k) => k.length));
  for (const [key, val] of Object.entries(data)) {
    console.log(`  ${key.padEnd(keyWidth)} : ${val}`);
  }
}

function resolveFaction(input: string, factions: Faction[]): Faction | null {
  const n = parseInt(input, 10);
  if (!isNaN(n) && n >= 1 && n <= factions.length) {
    return factions[n - 1];
  }
  if (factions.includes(input as Faction)) {
    return input as Faction;
  }
  return null;
}

function resolvePlaystyle(input: string, playstyles: Playstyle[]): Playstyle | null {
  const n = parseInt(input, 10);
  if (!isNaN(n) && n >= 1 && n <= playstyles.length) {
    return playstyles[n - 1];
  }
  if (playstyles.includes(input as Playstyle)) {
    return input as Playstyle;
  }
  return null;
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

const command = process.argv[2];

switch (command) {
  case "register":
    await cmdRegister();
    break;
  case "login":
    await cmdLogin();
    break;
  case "state":
    await cmdState();
    break;
  case "agent":
    await cmdAgent();
    break;
  default:
    console.log("GateWanderers CLI");
    console.log("");
    console.log("Usage:");
    console.log("  bun run src/index.ts <command>");
    console.log("");
    console.log("Commands:");
    console.log("  register   Create a new account and agent");
    console.log("  login      Log in and save your token");
    console.log("  state      Show your agent and ship state");
    console.log("  agent      Run the AI agent loop (connects via WebSocket)");
    process.exit(command ? 1 : 0);
}
