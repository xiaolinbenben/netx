import { useEffect, useMemo, useState } from "react";
import { App as AntApp, Button, ConfigProvider, Layout, Menu, Space, Typography, theme } from "antd";
import { GlobalOutlined, KeyOutlined, LogoutOutlined, SettingOutlined, TagsOutlined } from "@ant-design/icons";
import { clearToken, hasToken, setSessionLostHandler } from "./api";
import LoginPage from "./LoginPage";
import CodesPage from "./CodesPage";
import FetchProxyPage from "./FetchProxyPage";
import SettingsPage from "./SettingsPage";
const { Header, Sider, Content } = Layout;
export default function App() {
  const [authenticated, setAuthenticated] = useState(hasToken);
  const [sessionExpired, setSessionExpired] = useState(false);
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => {
    setSessionLostHandler(() => { setSessionExpired(true); setAuthenticated(false); });
    const handler = () => setPath(window.location.pathname);
    window.addEventListener("popstate", handler);
    return () => { window.removeEventListener("popstate", handler); setSessionLostHandler(() => {}); };
  }, []);
  const page = path.endsWith("/settings") ? "settings" : path.endsWith("/proxy") ? "proxy" : "codes";
  const go = (next: string) => { window.history.pushState({}, "", `/admin/${next}`); setPath(`/admin/${next}`); };
  const menuItems = useMemo(() => [{ key: "codes", icon: <TagsOutlined />, label: "兑换码" }, { key: "proxy", icon: <GlobalOutlined />, label: "订阅代理" }, { key: "settings", icon: <SettingOutlined />, label: "系统配置" }], []);
  if (!authenticated) return <ConfigProvider theme={{ algorithm: theme.defaultAlgorithm, token: { colorPrimary: "#1677ff" } }}><AntApp><LoginPage expired={sessionExpired} onSuccess={() => { setSessionExpired(false); setAuthenticated(true); }} /></AntApp></ConfigProvider>;
  return <ConfigProvider theme={{ algorithm: theme.defaultAlgorithm, token: { colorPrimary: "#1677ff", borderRadius: 6 } }}><AntApp><Layout className="admin-layout"><Sider breakpoint="lg" collapsedWidth="0" theme="light"><div className="admin-logo"><img src="/admin/netx-mark.png" alt="NetX" /><span>NetX 管理端</span></div><Menu mode="inline" selectedKeys={[page]} items={menuItems} onClick={({ key }) => go(key)} /></Sider><Layout><Header className="admin-header"><Typography.Title level={4}>NetX 控制台</Typography.Title><Space><span className="admin-user"><KeyOutlined /> 管理员</span><Button type="text" icon={<LogoutOutlined />} onClick={() => { clearToken(); setSessionExpired(false); setAuthenticated(false); }}>退出登录</Button></Space></Header><Content className="admin-content">{page === "settings" ? <SettingsPage /> : page === "proxy" ? <FetchProxyPage /> : <CodesPage />}</Content></Layout></Layout></AntApp></ConfigProvider>;
}
