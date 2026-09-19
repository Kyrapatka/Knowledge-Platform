export type User = { id: string; nickname: string; status: string };
type AuthResult = { user: User; access_token: string; expires_at: string };
let accessToken = "";
let refreshRequest: Promise<AuthResult> | null = null;
let authLost: (() => void) | null = null;
export const onAuthLost = (fn: () => void) => {
  authLost = fn;
};

const messages: Record<string, string> = {
  invalid_credentials: "The username or password is incorrect.",
  nickname_taken: "This username is already taken.",
  invalid_nickname: "Please use a username between 3 and 16 characters.",
  invalid_password: "Please use a password between 8 and 128 characters.",
  training_conflict: "The training state has changed. Refresh and try again.",
  folder_not_found: "This folder is no longer available.",
  invalid_values: "Please complete the required material fields.",
  invalid_metadata: "Please check the topic and metadata fields.",
  internal_error: "Something went wrong on the server. Please try again.",
  invalid_request: "Please check the information and try again.",
};
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message?: string,
    public data?: Record<string, unknown>,
  ) {
    super(message || messages[code] || code.replaceAll("_", " "));
  }
}
async function decode<T>(response: Response): Promise<T> {
  if (response.status === 204) return undefined as T;
  const data = await response.json().catch(() => ({}));
  if (!response.ok)
    throw new ApiError(
      response.status,
      data.error || "request_failed",
      data.message || messages[data.error],
      data,
    );
  return data as T;
}
async function rawAuth(kind: string, body?: unknown): Promise<AuthResult> {
  const data = await decode<AuthResult>(
    await fetch(`/api/v1/auth/browser/${kind}`, {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    }),
  );
  accessToken = data?.access_token || "";
  return data;
}
export function refreshAuth() {
  if (!refreshRequest) {
    const refresh = () => rawAuth("refresh");
    refreshRequest = (
      navigator.locks
        ? navigator.locks.request("knowledge-auth-refresh", refresh)
        : refresh()
    ).finally(() => {
      refreshRequest = null;
    });
  }
  return refreshRequest;
}
export const authenticate = (
  kind: "login" | "register",
  nickname: string,
  password: string,
) => rawAuth(kind, { nickname, password });
export async function logout() {
  if (navigator.locks)
    await navigator.locks.request("knowledge-auth-refresh", () =>
      rawAuth("logout"),
    );
  else await rawAuth("logout");
  accessToken = "";
}
export async function api<T>(
  path: string,
  options: { method?: string; body?: unknown; signal?: AbortSignal } = {},
  retried = false,
): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    method: options.method || "GET",
    signal: options.signal,
    credentials: "same-origin",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      ...(options.body !== undefined
        ? { "Content-Type": "application/json" }
        : {}),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
  if (response.status === 401 && !retried) {
    try {
      await refreshAuth();
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        accessToken = "";
        authLost?.();
      }
      throw error;
    }
    return api<T>(path, options, true);
  }
  return decode<T>(response);
}
export const post = <T>(path: string, body: unknown = {}) =>
  api<T>(path, { method: "POST", body });
export async function upload<T>(
  path: string,
  file: File,
  retried = false,
): Promise<T> {
  const form = new FormData();
  form.append("file", file, file.name);
  const response = await fetch(`/api/v1${path}`, {
    method: "POST",
    credentials: "same-origin",
    headers: { Authorization: `Bearer ${accessToken}` },
    body: form,
  });
  if (response.status === 401 && !retried) {
    try {
      await refreshAuth();
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        accessToken = "";
        authLost?.();
      }
      throw error;
    }
    return upload<T>(path, file, true);
  }
  return decode<T>(response);
}
export function errorText(error: unknown) {
  return error instanceof TypeError
    ? "Could not reach the server. Check your connection and try again."
    : error instanceof Error
      ? error.message
      : "Something went wrong. Please try again.";
}
export function readStored<T>(key: string): T | null {
  try {
    return JSON.parse(localStorage.getItem(key) || "null");
  } catch {
    return null;
  }
}
export function saveStored(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* The server still retains all confirmed progress. */
  }
}
