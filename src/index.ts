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
 * Call the local Ollama API and return the action string (e.g. "EXPLORE").
 * Falls back to "EXPLORE" if the model response cannot be parsed.
 */
async function askOllama(
  ollamaURL: string,
  model: string,
  prompt: string
): Promise<string> {
  try {
    const res = await fetch(`${ollamaURL.replace(/\/$/, "")}/api/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        model,
        stream: false,
        messages: [{ role: "user", content: prompt }],
      }),
    });
    if (!res.ok) throw new Error(`Ollama HTTP ${res.status}`);
    const data = await res.json() as { message?: { content?: string } };
    const content = data.message?.content ?? "";
    // Try to extract a JSON object with an "action" field.
    const match = content.match(/\{[^}]*"action"\s*:\s*"([^"]+)"[^}]*\}/);
    if (match) return match[1].toUpperCase();
  } catch (err) {
    console.error("Ollama error (defaulting to EXPLORE):", (err as Error).message);
  }
  return "EXPLORE";
}

/**
 * Connect to the game stream and run the AI agent loop.
 * Automatically reconnects after 5 seconds on disconnect.
 */
async function cmdAgent(): Promise<void> {
  const config = loadConfig();

  if (!config.token) {
    console.error("No token found. Please run 'login' or 'register' first.");
    process.exit(1);
  }

  const client = new ApiClient(config.server_url, config.token);
  let lang: string = "en"; // resolved from agent state on first tick

  console.log("=== GateWanderers — AI Agent ===");
  console.log("Connecting to", config.server_url, "...");

  // eslint-disable-next-line no-constant-condition
  while (true) {
    await new Promise<void>((resolve) => {
      const url = buildStreamURL(config.server_url, config.token);
      const ws = new WebSocket(url);

      ws.onopen = () => {
        console.log("Connected to game stream.");
      };

      ws.onmessage = async (event: MessageEvent) => {
        let msg: Record<string, unknown>;
        try {
          msg = JSON.parse(event.data as string) as Record<string, unknown>;
        } catch {
          return;
        }

        const msgType = msg["type"] as string | undefined;

        if (msgType === "tick") {
          const tick = msg["tick"] as number;
          console.log(`\n--- Tick ${tick} ---`);

          // Fetch current agent state to build the prompt.
          let prompt: string;
          try {
            const state = await client.getAgentState();
            const { agent, ship } = state;
            prompt = [
              "You are an AI agent in GateWanderers, a space MMO in the Stargate universe.",
              `Your faction: ${agent.faction}`,
              `Your playstyle: ${agent.playstyle}`,
              `Your ship: ${ship.name} at ${ship.galaxy_id}/${ship.system_id}`,
              `Mission brief: ${agent.mission_brief || "(none)"}`,
              "",
              "Available actions: EXPLORE",
              "",
              'Respond with ONLY a JSON object: {"action": "EXPLORE"}',
            ].join("\n");
          } catch (err) {
            console.error("Failed to fetch agent state:", (err as Error).message);
            prompt = 'Respond with ONLY a JSON object: {"action": "EXPLORE"}';
          }

          const action = await askOllama(
            config.ollama_url,
            config.ollama_model,
            prompt
          );
          console.log(`AI chose action: ${action}`);

          try {
            await client.submitAction(action);
            console.log(`Action ${action} queued for next tick.`);
          } catch (err) {
            console.error("Failed to submit action:", (err as Error).message);
          }
        } else if (msgType === "event") {
          const eventData = msg["event"] as Record<string, unknown> | undefined;
          if (eventData) {
            const payload =
              lang === "de"
                ? (eventData["payload_de"] as string)
                : (eventData["payload_en"] as string);
            console.log(`[Event] ${payload}`);
          }
        } else if (msgType === "connected") {
          console.log(`Stream ready. Current tick: ${msg["tick"]}`);
        }
      };

      ws.onerror = (err: Event) => {
        console.error("WebSocket error:", err);
      };

      ws.onclose = () => {
        console.log("WebSocket disconnected. Reconnecting in 5 seconds...");
        setTimeout(() => resolve(), 5000);
      };
    });
  }
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
