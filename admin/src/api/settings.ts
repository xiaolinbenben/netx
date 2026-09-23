import { http } from "@/utils/http";

export type SettingField = {
  key: string;
  label: string;
  type: "text" | "textarea" | "bool";
  secret: boolean;
  value: string;
  configured: boolean;
  hint: string;
  placeholder: string;
};

export type SettingGroup = {
  key: string;
  title: string;
  fields: SettingField[];
};

type Result<T> = {
  success: boolean;
  message?: string;
  data: T;
};

/** 读取系统配置 */
export const getSettings = () => {
  return http.request<Result<{ groups: SettingGroup[] }>>(
    "get",
    "/api/admin/settings"
  );
};

/** 保存系统配置 */
export const saveSettings = (values: Record<string, string>) => {
  return http.request<Result<null>>("put", "/api/admin/settings", {
    data: { values }
  });
};
