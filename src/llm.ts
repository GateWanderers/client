/**
 * LLM provider abstraction for GateWanderers.
 * Add new providers by implementing the LLMProvider interface.
 */

import type { AgentState, LLMConfig } from "./types.ts";

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

export interface LLMProvider {
  name: string;
  complete(prompt: string, context: AgentState): Promise<string>;
}

// ---------------------------------------------------------------------------
// Shared helper: extract action from LLM response text
// ---------------------------------------------------------------------------

function extractAction(content: string): string {
  const match = content.match(/\{[^}]*"action"\s*:\s*"([^"]+)"[^}]*\}/);
  if (match) return match[1].toUpperCase();
  return "EXPLORE";
}

// ---------------------------------------------------------------------------
// Ollama
// ---------------------------------------------------------------------------

export class OllamaProvider implements LLMProvider {
  name = "ollama";

  constructor(private baseURL: string, private model: string) {}

  async complete(prompt: string, _context: AgentState): Promise<string> {
    const res = await fetch(`${this.baseURL.replace(/\/$/, "")}/api/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        model: this.model,
        stream: false,
        messages: [{ role: "user", content: prompt }],
      }),
    });
    if (!res.ok) throw new Error(`Ollama HTTP ${res.status}`);
    const data = (await res.json()) as { message?: { content?: string } };
    return extractAction(data.message?.content ?? "");
  }
}

// ---------------------------------------------------------------------------
// OpenAI-compatible (works with OpenAI, Groq, LM Studio, etc.)
// ---------------------------------------------------------------------------

export class OpenAIProvider implements LLMProvider {
  name = "openai";

  constructor(
    private baseURL: string,
    private model: string,
    private apiKey: string
  ) {}

  async complete(prompt: string, _context: AgentState): Promise<string> {
    const url = `${this.baseURL.replace(/\/$/, "")}/chat/completions`;
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${this.apiKey}`,
      },
      body: JSON.stringify({
        model: this.model,
        messages: [{ role: "user", content: prompt }],
        max_tokens: 64,
        temperature: 0.3,
      }),
    });
    if (!res.ok) throw new Error(`OpenAI HTTP ${res.status}`);
    const data = (await res.json()) as {
      choices?: Array<{ message?: { content?: string } }>;
    };
    const content = data.choices?.[0]?.message?.content ?? "";
    return extractAction(content);
  }
}

// ---------------------------------------------------------------------------
// Anthropic Claude
// ---------------------------------------------------------------------------

export class AnthropicProvider implements LLMProvider {
  name = "anthropic";
  private static readonly API_URL = "https://api.anthropic.com/v1/messages";

  constructor(private model: string, private apiKey: string) {}

  async complete(prompt: string, _context: AgentState): Promise<string> {
    const res = await fetch(AnthropicProvider.API_URL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "x-api-key": this.apiKey,
        "anthropic-version": "2023-06-01",
      },
      body: JSON.stringify({
        model: this.model,
        max_tokens: 64,
        messages: [{ role: "user", content: prompt }],
      }),
    });
    if (!res.ok) throw new Error(`Anthropic HTTP ${res.status}`);
    const data = (await res.json()) as {
      content?: Array<{ type: string; text?: string }>;
    };
    const text = data.content?.find((b) => b.type === "text")?.text ?? "";
    return extractAction(text);
  }
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

export function createProvider(cfg: LLMConfig): LLMProvider {
  switch (cfg.provider) {
    case "ollama":
      return new OllamaProvider(
        cfg.base_url ?? "http://localhost:11434",
        cfg.model
      );
    case "openai":
      return new OpenAIProvider(
        cfg.base_url ?? "https://api.openai.com/v1",
        cfg.model,
        cfg.api_key ?? ""
      );
    case "groq":
      return new OpenAIProvider(
        cfg.base_url ?? "https://api.groq.com/openai/v1",
        cfg.model,
        cfg.api_key ?? ""
      );
    case "anthropic":
      return new AnthropicProvider(cfg.model, cfg.api_key ?? "");
    default:
      throw new Error(`Unknown LLM provider: ${(cfg as LLMConfig).provider}`);
  }
}
