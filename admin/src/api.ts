export type CodeStatus = "unused" | "reserved" | "used" | "void";
export type RedeemCode = { id: number; code: string; note: string; status: CodeStatus; createdAt: string; usedAt: string; plan: string; subscriptionUrl: string };
export type SettingField = { key: string; label: string; type: "text" | "textarea"; secret: boolean; value: string; configured: boolean; hint: string; placeholder: string };
export type SettingGroup = { key: string; title: string; fields: SettingField[] };
type Envelope<T> = { success: boolean; message?: string; data: T };
const tokenKey = "netx-admin-token";
const getToken = () => JSON.parse(localStorage.getItem(tokenKey) || "null") as { accessToken: string; refreshToken: string; expires: string } | null;
export const saveToken = (value: unknown) => localStorage.setItem(tokenKey, JSON.stringify(value));
export const clearToken = () => localStorage.removeItem(tokenKey);
export const hasToken = () => Boolean(getToken()?.accessToken);
async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers); headers.set("Content-Type", "application/json");
  const token = getToken(); if (token?.accessToken) headers.set("Authorization", `Bearer ${token.accessToken}`);
  const response = await fetch(url, { ...init, headers });
  const payload = await response.json().catch(() => ({})) as Partial<Envelope<T>>;
  if (!response.ok || payload.success === false) throw new Error(payload.message || "请求失败");
  return payload.data as T;
}
export const login = (username: string, password: string) => request<{ accessToken: string; refreshToken: string; expires: string; username: string }>("/api/admin/login", { method: "POST", body: JSON.stringify({ username, password }) });
export const listCodes = (params: Record<string, string | number>) => request<{ items: RedeemCode[]; total: number }>(`/api/admin/codes?${new URLSearchParams(Object.entries(params).map(([key, value]) => [key, String(value)]))}`);
export const generateCodes = (data: { note: string; plan: string; subscriptionUrl: string }) => request<{ items: RedeemCode[] }>("/api/admin/codes", { method: "POST", body: JSON.stringify(data) });
export const updateCodeStatus = (id: number, status: CodeStatus) => request<null>(`/api/admin/codes/${id}`, { method: "PATCH", body: JSON.stringify({ status }) });
export const getSettings = () => request<{ groups: SettingGroup[] }>("/api/admin/settings");
export const saveSettings = (values: Record<string, string>) => request<null>("/api/admin/settings", { method: "PUT", body: JSON.stringify({ values }) });
