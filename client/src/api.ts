import type {
  AgentState,
  ErrorResponse,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
} from "./types.ts";

/**
 * Retryable HTTP error: 5xx or network failure.
 */
function isRetryable(err: unknown): boolean {
  if (err instanceof RetryableError) return true;
  // Network-level errors (fetch throws TypeError on connection refused etc.)
  if (err instanceof TypeError) return true;
  return false;
}

class RetryableError extends Error {
  constructor(status: number) {
    super(`HTTP ${status}`);
    this.name = "RetryableError";
  }
}

/**
 * Retry fn up to maxRetries times with exponential backoff (1s, 2s, 4s … max 60s).
 * Permanent errors (4xx, auth) are not retried.
 */
async function withRetry<T>(fn: () => Promise<T>, maxRetries = 5): Promise<T> {
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      return await fn();
    } catch (err) {
      if (!isRetryable(err) || attempt === maxRetries) throw err;
      const delay = Math.min(1000 * 2 ** attempt, 60_000);
      console.error(`[retry] attempt ${attempt + 1}/${maxRetries} failed — retrying in ${delay / 1000}s`);
      await new Promise((r) => setTimeout(r, delay));
    }
  }
  throw new Error("Max retries exceeded");
}

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
    return withRetry(async () => {
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

      // 5xx → retryable; 4xx → permanent error
      if (response.status >= 500) {
        throw new RetryableError(response.status);
      }

      const data = await response.json();

      if (!response.ok) {
        const err = data as ErrorResponse;
        throw new Error(err.error ?? `HTTP ${response.status}`);
      }

      return data as T;
    });
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
