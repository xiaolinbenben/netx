export type CodeStatus = "unused" | "reserved" | "used" | "void";
export type RedeemCode = { id: number; code: string; note: string; status: CodeStatus; createdAt: string; usedAt: string; plan: string; subscriptionUrl: string };
export type SettingField = { key: string; label: string; type: "text" | "textarea"; secret: boolean; value: string; configured: boolean; hint: string; placeholder: string };
export type SettingGroup = { key: string; title: string; fields: SettingField[] };
type Envelope<T> = { success: boolean; message?: string; data: T };
type Session = { accessToken?: string; refreshToken?: string };
const tokenKey = "netx-admin-token";
const readSession = () => JSON.parse(localStorage.getItem(tokenKey) || "null") as Session | null;
export const saveToken = (value: unknown) => localStorage.setItem(tokenKey, JSON.stringify(value));
export const clearToken = () => localStorage.removeItem(tokenKey);
export const hasToken = () => Boolean(readSession()?.accessToken);
let onSessionLost: (() => void) | null = null;
export const setSessionLostHandler = (handler: () => void) => { onSessionLost = handler; };

async function refreshSession() {
  const current = readSession();
  if (!current?.refreshToken) return false;
  const response = await fetch("/api/admin/refresh-token", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ refreshToken: current.refreshToken }) });
  const payload = await response.json().catch(() => ({})) as Partial<Envelope<Session>>;
  if (!response.ok || payload.success === false || !payload.data?.accessToken) return false;
  saveToken(payload.data);
  return true;
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const run = async () => {
    const headers = new Headers(init.headers);
    headers.set("Content-Type", "application/json");
    const token = readSession();
    if (token?.accessToken) headers.set("Authorization", `Bearer ${token.accessToken}`);
    const response = await fetch(url, { ...init, headers });
    const payload = await response.json().catch(() => ({})) as Partial<Envelope<T>>;
    return { response, payload };
  };
  let { response, payload } = await run();
  if (response.status === 401 && url !== "/api/admin/login") {
    if (await refreshSession().catch(() => false)) ({ response, payload } = await run());
    if (response.status === 401) { clearToken(); onSessionLost?.(); throw new Error(payload.message || "登录已过期，请重新登录"); }
  }
  if (!response.ok || payload.success === false) throw new Error(payload.message || "请求失败");
  return payload.data as T;
}

export const login = (username: string, password: string) => request<{ accessToken: string; refreshToken: string; username: string }>("/api/admin/login", { method: "POST", body: JSON.stringify({ username, password }) });
export const listCodes = (params: Record<string, string | number>) => request<{ items: RedeemCode[]; total: number }>(`/api/admin/codes?${new URLSearchParams(Object.entries(params).map(([key, value]) => [key, String(value)]))}`);
export const generateCodes = (data: { note: string; plan: string; subscriptionUrl: string }) => request<{ items: RedeemCode[] }>("/api/admin/codes", { method: "POST", body: JSON.stringify(data) });
export const updateCodeStatus = (id: number, status: CodeStatus) => request<null>(`/api/admin/codes/${id}`, { method: "PATCH", body: JSON.stringify({ status }) });
export const updateCode = (id: number, data: { note: string; plan: string; subscriptionUrl: string }) => request<null>(`/api/admin/codes/${id}`, { method: "PUT", body: JSON.stringify(data) });
export const deleteCode = (id: number) => request<null>(`/api/admin/codes/${id}`, { method: "DELETE" });
export const getSettings = () => request<{ groups: SettingGroup[] }>("/api/admin/settings");
export const saveSettings = (values: Record<string, string>) => request<null>("/api/admin/settings", { method: "PUT", body: JSON.stringify({ values }) });
export type FetchProxy = { configured: boolean; enabled: boolean; host: string; port: number; username: string; passwordConfigured: boolean; passwordHint: string };
export type FetchProxyInput = { enabled: boolean; host: string; port: number; username: string; password: string; updatePassword: boolean };
export const getFetchProxy = () => request<FetchProxy>("/api/admin/fetch-proxy");
export const saveFetchProxy = (data: FetchProxyInput) => request<null>("/api/admin/fetch-proxy", { method: "PUT", body: JSON.stringify(data) });
export const deleteFetchProxy = () => request<null>("/api/admin/fetch-proxy", { method: "DELETE" });
export const parseFetchProxy = (link: string) => request<{ host: string; port: number; username: string; password: string }>("/api/admin/fetch-proxy/parse", { method: "POST", body: JSON.stringify({ link }) });
export const testFetchProxy = (data: { host: string; port: number; username: string; password: string }) => request<{ elapsedMs: number }>("/api/admin/fetch-proxy/test", { method: "POST", body: JSON.stringify(data) });
