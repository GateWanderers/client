// Shared TypeScript types for the GateWanderers CLI client.

export type Faction =
  | "tau_ri"
  | "free_jaffa"
  | "gate_nomad"
  | "system_lord_remnant"
  | "wraith_brood"
  | "ancient_seeker";

export type Playstyle = "fighter" | "trader" | "researcher";

export type ShipClass = "gate_runner_mk1";

export interface Agent {
  id: string;
  account_id: string;
  name: string;
  faction: Faction;
  playstyle: Playstyle;
  credits: number;
  experience: number;
  skills: Record<string, unknown>;
  research: unknown[];
  reputation: Record<string, unknown>;
  mission_brief: string;
  created_at: string;
}

export interface Ship {
  id: string;
  agent_id: string;
  name: string;
  class: ShipClass;
  hull_points: number;
  max_hull_points: number;
  galaxy_id: string;
  system_id: string;
  planet_id: string | null;
  equipment: unknown[];
  created_at: string;
}

export interface AgentState {
  agent: Agent;
  ship: Ship;
}

export interface RegisterRequest {
  email: string;
  password: string;
  agent_name: string;
  faction: Faction;
  playstyle: Playstyle;
  language: string;
}

export interface RegisterResponse {
  token: string;
  agent: Agent;
}

export interface LoginResponse {
  token: string;
}

export interface ErrorResponse {
  error: string;
}

export interface LLMConfig {
  provider: "ollama" | "openai" | "anthropic" | "groq";
  model: string;
  base_url?: string;
  api_key?: string;
}

export interface Config {
  server_url: string;
  token: string;
  /** LLM provider configuration. Replaces the legacy ollama_url/ollama_model fields. */
  llm?: LLMConfig;
  /** Maximum number of retries for transient API errors. Default: 5. */
  max_retries?: number;
  /** @deprecated Use llm.base_url instead. */
  ollama_url?: string;
  /** @deprecated Use llm.model instead. */
  ollama_model?: string;
}
