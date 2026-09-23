import { http } from "@/utils/http";

export type UserResult = {
  success: boolean;
  message?: string;
  data: {
    /** 头像 */
    avatar: string;
    /** 用户名 */
    username: string;
    /** 昵称 */
    nickname: string;
    /** 当前登录用户的角色 */
    roles: Array<string>;
    /** 按钮级别权限 */
    permissions: Array<string>;
    /** `token` */
    accessToken: string;
    /** 用于刷新`accessToken`的接口所需的`token` */
    refreshToken: string;
    /** `accessToken`的过期时间 */
    expires: string;
  };
};

export type RefreshTokenResult = {
  success: boolean;
  message?: string;
  data: {
    accessToken: string;
    refreshToken: string;
    expires: string;
  };
};

/** 登录 */
export const getLogin = (data?: object) => {
  return http.request<UserResult>("post", "/api/admin/login", { data });
};

/** 刷新`token` */
export const refreshTokenApi = (data?: object) => {
  return http.request<RefreshTokenResult>("post", "/api/admin/refresh-token", {
    data
  });
};
