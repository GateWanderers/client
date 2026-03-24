import type {
  AgentState,
  ErrorResponse,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
} from "./types.ts";

/**
 * GateWanderers typed API client.
 */
export class ApiClient {
  private baseURL: string;
  private token: string;

  constructor(baseURL: string, token = "") {
    this.baseURL = baseURL.replace(/\/$/, "");
    this.token = token;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    auth = false
  ): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };

    if (auth) {
      if (!this.token) {
        throw new Error("Not authenticated. Please run 'login' first.");
      }
      headers["Authorization"] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${this.baseURL}${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    const data = await response.json();

    if (!response.ok) {
      const err = data as ErrorResponse;
      throw new Error(err.error ?? `HTTP ${response.status}`);
    }

    return data as T;
  }

  /**
   * Register a new account and agent.
   */
  async register(req: RegisterRequest): Promise<RegisterResponse> {
    return this.request<RegisterResponse>("POST", "/auth/register", req);
  }

  /**
   * Log in with email and password; returns a PASETO token.
   */
  async login(email: string, password: string): Promise<LoginResponse> {
    return this.request<LoginResponse>("POST", "/auth/login", {
      email,
      password,
    });
  }

  /**
   * Fetch the authenticated account's full agent + ship state.
   */
  async getAgentState(): Promise<AgentState> {
    return this.request<AgentState>("GET", "/agent/state", undefined, true);
  }

  /**
   * Submit an action to the server for the next tick.
   */
  async submitAction(type: string): Promise<void> {
    await this.request<{ queued: boolean; action: string }>(
      "POST",
      "/agent/action",
      { type },
      true
    );
  }

  /** Update the bearer token used for authenticated requests. */
  setToken(token: string): void {
    this.token = token;
  }
}

/**
 * Build a WebSocket URL for /stream using the token as a query param.
 * Converts http:// → ws:// and https:// → wss://.
 */
export function buildStreamURL(serverURL: string, token: string): string {
  const base = serverURL.replace(/\/$/, "").replace(/^http/, "ws");
  return `${base}/stream?token=${encodeURIComponent(token)}`;
}
