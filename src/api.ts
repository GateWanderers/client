import type {
  AgentState,
  ErrorResponse,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
} from "./types.ts";

// ---------------------------------------------------------------------------
// Error types
// ---------------------------------------------------------------------------

/** Thrown when the server responds with a non-2xx status. */
export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// ---------------------------------------------------------------------------
// Retry helpers
// ---------------------------------------------------------------------------

/**
 * Returns true if the error is transient and worth retrying:
 * - Network / fetch-level errors (TypeError, ECONNREFUSED, …)
 * - HTTP 429 Too Many Requests
 * - HTTP 5xx Server Errors
 *
 * Permanent errors (4xx except 429) are NOT retried.
 */
export function isRetryable(err: unknown): boolean {
  if (err instanceof ApiError) {
    return err.status === 429 || err.status >= 500;
  }
  // Network-level error (no response received)
  return err instanceof TypeError;
}

/**
 * Calls `fn` up to `maxRetries` times, applying exponential backoff
 * (1 s, 2 s, 4 s, … capped at 60 s) between attempts.
 * Logs each retry attempt to stderr with the given `label`.
 */
export async function withRetry<T>(
  fn: () => Promise<T>,
  maxRetries = 5,
  label = "request"
): Promise<T> {
  let lastErr: unknown;
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      return await fn();
    } catch (err) {
      lastErr = err;
      if (!isRetryable(err) || attempt === maxRetries) throw err;
      const delayMs = Math.min(1000 * 2 ** attempt, 60_000);
      console.error(
        `[retry] ${label} failed (${(err as Error).message}). ` +
          `Attempt ${attempt + 1}/${maxRetries}, retrying in ${(delayMs / 1000).toFixed(0)}s…`
      );
      await new Promise((r) => setTimeout(r, delayMs));
    }
  }
  throw lastErr;
}

// ---------------------------------------------------------------------------
// API client
// ---------------------------------------------------------------------------

/**
 * GateWanderers typed API client.
 */
export class ApiClient {
  private baseURL: string;
  private token: string;
  private maxRetries: number;

  constructor(baseURL: string, token = "", maxRetries = 5) {
    this.baseURL = baseURL.replace(/\/$/, "");
    this.token = token;
    this.maxRetries = maxRetries;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    auth = false,
    retry = false
  ): Promise<T> {
    const doRequest = async (): Promise<T> => {
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
        throw new ApiError(err.error ?? `HTTP ${response.status}`, response.status);
      }

      return data as T;
    };

    if (retry) {
      return withRetry(doRequest, this.maxRetries, `${method} ${path}`);
    }
    return doRequest();
  }

  /**
   * Register a new account and agent. No retry (interactive, no point retrying 4xx).
   */
  async register(req: RegisterRequest): Promise<RegisterResponse> {
    return this.request<RegisterResponse>("POST", "/auth/register", req);
  }

  /**
   * Log in with email and password; returns a PASETO token. No retry.
   */
  async login(email: string, password: string): Promise<LoginResponse> {
    return this.request<LoginResponse>("POST", "/auth/login", {
      email,
      password,
    });
  }

  /**
   * Fetch the authenticated account's full agent + ship state.
   * Retries on transient errors.
   */
  async getAgentState(): Promise<AgentState> {
    return this.request<AgentState>("GET", "/agent/state", undefined, true, true);
  }

  /**
   * Submit an action to the server for the next tick.
   * Retries on transient errors.
   */
  async submitAction(type: string): Promise<void> {
    await this.request<{ queued: boolean; action: string }>(
      "POST",
      "/agent/action",
      { type },
      true,
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
