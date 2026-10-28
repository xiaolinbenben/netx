import { useEffect, useState } from "react";
import { App, Button, Card, Form, Input, InputNumber, Skeleton, Space, Switch, Typography } from "antd";
import { ApiOutlined, DeleteOutlined, SaveOutlined } from "@ant-design/icons";
import { deleteFetchProxy, getFetchProxy, parseFetchProxy, saveFetchProxy, testFetchProxy, type FetchProxy } from "./api";

type FormValues = { enabled: boolean; host: string; port: number | null; username: string; password: string };
const emptyProxy: FetchProxy = { configured: false, enabled: false, host: "", port: 0, username: "", passwordConfigured: false, passwordHint: "" };

export default function FetchProxyPage() {
  const [proxy, setProxy] = useState<FetchProxy | null>(null);
  const [link, setLink] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [parsing, setParsing] = useState(false);
  const [testing, setTesting] = useState(false);
  const [updatePassword, setUpdatePassword] = useState(false);
  const [form] = Form.useForm<FormValues>();
  const { message, modal } = App.useApp();

  async function load() {
    setLoading(true);
    try {
      const data = await getFetchProxy();
      setProxy(data);
      form.setFieldsValue({ enabled: data.enabled, host: data.host, port: data.port || null, username: data.username, password: "" });
      setUpdatePassword(false);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "加载订阅代理失败");
      setProxy((current) => current ?? emptyProxy);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, []);

  async function recognize() {
    setParsing(true);
    try {
      const parsed = await parseFetchProxy(link);
      form.setFieldsValue({ host: parsed.host, port: parsed.port, username: parsed.username, password: parsed.password });
      setUpdatePassword(true);
      message.success("已识别 SOCKS5 链接");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "识别 SOCKS5 链接失败");
    } finally {
      setParsing(false);
    }
  }

  async function submit(values: FormValues) {
    setSaving(true);
    try {
      await saveFetchProxy({ enabled: values.enabled, host: values.host || "", port: values.port || 0, username: values.username || "", password: values.password || "", updatePassword });
      message.success("订阅代理已保存");
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "保存订阅代理失败");
    } finally {
      setSaving(false);
    }
  }

  async function test() {
    let values: FormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setTesting(true);
    try {
      const result = await testFetchProxy({ host: values.host, port: values.port || 0, username: values.username || "", password: values.password || "" });
      message.success(`连接成功，耗时 ${result.elapsedMs} ms`);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "连接 SOCKS5 失败");
    } finally {
      setTesting(false);
    }
  }

  function clear() {
    modal.confirm({
      title: "清空订阅代理",
      content: "清空后，订阅拉取不再使用 SOCKS5 兜底。",
      okText: "清空",
      cancelText: "取消",
      onOk: async () => {
        await deleteFetchProxy();
        message.success("订阅代理已清空");
        setLink("");
        await load();
      },
    });
  }

  if (loading || !proxy) return <Skeleton active />;
  const passwordPlaceholder = proxy.passwordConfigured ? `已配置（${proxy.passwordHint}），留空表示不修改` : "没有认证可以留空";
  return <section>
    <div className="page-heading">
      <div>
        <Typography.Title level={3}>订阅代理</Typography.Title>
        <Typography.Paragraph type="secondary">直连订阅失败时，通过这组 SOCKS5 再请求一次。代理地址需要从本机能够连通。</Typography.Paragraph>
      </div>
      <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={() => form.submit()}>保存配置</Button>
    </div>
    <Card title="识别链接" className="settings-card">
      <Space.Compact style={{ width: "100%" }}>
        <Input value={link} onChange={(event) => setLink(event.target.value)} placeholder="socks5://用户名:密码@地址:端口" onPressEnter={() => void recognize()} />
        <Button loading={parsing} onClick={() => void recognize()}>识别</Button>
      </Space.Compact>
    </Card>
    <Card title="代理参数" className="settings-card">
      <Form form={form} layout="vertical" onFinish={submit} initialValues={{ enabled: false }}>
        <Form.Item name="enabled" label="启用兜底" valuePropName="checked"><Switch /></Form.Item>
        <Form.Item name="host" label="地址" rules={[{ required: true, message: "请填写代理地址" }]}><Input placeholder="203.0.113.10" /></Form.Item>
        <Form.Item name="port" label="端口" rules={[{ required: true, message: "请填写端口" }]}><InputNumber min={1} max={65535} precision={0} style={{ width: "100%" }} placeholder="1080" /></Form.Item>
        <Form.Item name="username" label="用户名"><Input placeholder="没有认证可以留空" autoComplete="off" /></Form.Item>
        <Form.Item name="password" label="密码"><Input.Password placeholder={passwordPlaceholder} autoComplete="new-password" onChange={() => setUpdatePassword(true)} /></Form.Item>
        <Space>
          <Button icon={<ApiOutlined />} loading={testing} onClick={() => void test()}>测试连接</Button>
          <Button icon={<DeleteOutlined />} disabled={!proxy.configured} onClick={clear}>清空</Button>
        </Space>
      </Form>
    </Card>
  </section>;
}
