import { join } from "node:path";
import { readFileSync, writeFileSync, mkdirSync, existsSync } from "node:fs";
import type { Config, LLMConfig } from "./types.ts";

const CONFIG_DIR = join(process.env.HOME ?? "~", ".gatewanderers");
const CONFIG_FILE = join(CONFIG_DIR, "config.json");

const DEFAULT_LLM: LLMConfig = {
  provider: "ollama",
  model: "llama3.1:8b",
  base_url: "http://localhost:11434",
};

const DEFAULTS: Config = {
  server_url: "http://localhost:8080",
  token: "",
  llm: DEFAULT_LLM,
};

/**
 * Load the config file from ~/.gatewanderers/config.json.
 * Returns defaults merged with any values found on disk.
 */
export function loadConfig(): Config {
  if (!existsSync(CONFIG_FILE)) {
    return { ...DEFAULTS };
  }
  try {
    const raw = readFileSync(CONFIG_FILE, "utf-8");
    const parsed = JSON.parse(raw) as Partial<Config>;
    const merged: Config = { ...DEFAULTS, ...parsed };
    // Migrate legacy ollama_url / ollama_model to llm config.
    if (!merged.llm && (merged.ollama_url || merged.ollama_model)) {
      merged.llm = {
        provider: "ollama",
        model: merged.ollama_model ?? DEFAULT_LLM.model,
        base_url: merged.ollama_url ?? DEFAULT_LLM.base_url,
      };
    }
    merged.llm ??= DEFAULT_LLM;
    return merged;
  } catch {
    console.error(`Warning: could not parse ${CONFIG_FILE}, using defaults.`);
    return { ...DEFAULTS };
  }
}

/**
 * Persist the given config object to ~/.gatewanderers/config.json.
 */
export function saveConfig(config: Config): void {
  if (!existsSync(CONFIG_DIR)) {
    mkdirSync(CONFIG_DIR, { recursive: true });
  }
  writeFileSync(CONFIG_FILE, JSON.stringify(config, null, 2) + "\n", "utf-8");
}

/**
 * Save a new token into the existing config file.
 */
export function saveToken(token: string): void {
  const config = loadConfig();
  config.token = token;
  saveConfig(config);
}
