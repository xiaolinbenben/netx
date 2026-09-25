import { useState } from "react";
import { App, Button, Card, Form, Input, Typography } from "antd";
import { LockOutlined, UserOutlined } from "@ant-design/icons";
import { login, saveToken } from "./api";
export default function LoginPage({ onSuccess }: { onSuccess: () => void }) {
  const [loading, setLoading] = useState(false); const { message } = App.useApp();
  async function submit(values: { username: string; password: string }) { setLoading(true); try { const data = await login(values.username, values.password); saveToken(data); window.history.replaceState({}, "", "/admin/codes"); onSuccess(); message.success("登录成功"); } catch (error) { message.error(error instanceof Error ? error.message : "登录失败"); } finally { setLoading(false); } }
  return <main className="login-page"><Card className="login-card" bordered={false}><div className="login-brand"><img className="login-mark" src="/admin/netx-mark.png" alt="NetX" /><div><Typography.Title level={2}>NetX 管理端</Typography.Title><Typography.Text type="secondary">企业网络服务控制台</Typography.Text></div></div><Form layout="vertical" size="large" onFinish={submit} requiredMark={false}><Form.Item name="username" label="管理员账号" rules={[{ required: true, message: "请输入管理员账号" }]}><Input prefix={<UserOutlined />} placeholder="请输入账号" autoComplete="username" /></Form.Item><Form.Item name="password" label="密码" rules={[{ required: true, message: "请输入密码" }]}><Input.Password prefix={<LockOutlined />} placeholder="请输入密码" autoComplete="current-password" /></Form.Item><Button type="primary" htmlType="submit" block loading={loading}>登录</Button></Form></Card></main>;
}
