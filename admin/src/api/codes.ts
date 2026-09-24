import { http } from "@/utils/http";

export type CodeStatus = "unused" | "reserved" | "used" | "void";

export type RedeemCode = {
  id: number;
  code: string;
  note: string;
  status: CodeStatus;
  createdAt: string;
  usedAt: string;
  plan: string;
  subscriptionUrl: string;
};

type CodeListData = {
  items: RedeemCode[];
  total: number;
};

type GenerateData = {
  items: RedeemCode[];
};

type Result<T> = {
  success: boolean;
  message?: string;
  data: T;
};

/** 兑换码列表 */
export const listCodes = (params?: object) => {
  return http.request<Result<CodeListData>>("get", "/api/admin/codes", {
    params
  });
};

/** 批量生成兑换码 */
export const generateCodes = (data: { note: string; plan: string; subscriptionUrl: string }) => {
  return http.request<Result<GenerateData>>("post", "/api/admin/codes", {
    data
  });
};

/** 作废或恢复兑换码 */
export const updateCodeStatus = (id: number, status: CodeStatus) => {
  return http.request<Result<null>>("patch", `/api/admin/codes/${id}`, {
    data: { status }
  });
};
