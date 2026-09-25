import { useEffect, useState } from "react";
import { App, Button, Card, Form, Input, Skeleton, Typography } from "antd";
import { SaveOutlined } from "@ant-design/icons";
import { getSettings, saveSettings, type SettingGroup } from "./api";
export default function SettingsPage() {
  const [groups, setGroups] = useState<SettingGroup[]>([]); const [loading, setLoading] = useState(true); const [saving, setSaving] = useState(false); const [form] = Form.useForm<Record<string, string>>(); const { message } = App.useApp();
  async function load() { setLoading(true); try { const data = await getSettings(); setGroups(data.groups); const values: Record<string, string> = {}; data.groups.forEach(group => group.fields.forEach(field => { values[field.key] = field.secret ? "" : field.value || ""; })); form.setFieldsValue(values); } catch (error) { message.error(error instanceof Error ? error.message : "加载系统配置失败"); } finally { setLoading(false); } }
  useEffect(() => { void load(); }, []);
  async function submit(values: Record<string, string>) { setSaving(true); try { const changed: Record<string, string> = {}; groups.forEach(group => group.fields.forEach(field => { if (!field.secret || values[field.key]) changed[field.key] = values[field.key] || ""; })); await saveSettings(changed); message.success("配置已保存"); await load(); } catch (error) { message.error(error instanceof Error ? error.message : "保存系统配置失败"); } finally { setSaving(false); } }
  if (loading) return <Skeleton active />;
  return <section><div className="page-heading"><div><Typography.Title level={3}>系统配置</Typography.Title><Typography.Paragraph type="secondary">配置项目运行地址和支付宝支付参数。</Typography.Paragraph></div><Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={() => form.submit()}>保存配置</Button></div><Form form={form} layout="vertical" onFinish={submit}>{groups.map(group => <Card key={group.key} title={group.title} className="settings-card">{group.fields.map(field => <Form.Item key={field.key} name={field.key} label={field.label}><Input.TextArea autoSize={{ minRows: field.type === "textarea" ? 4 : 1, maxRows: 8 }} placeholder={field.secret ? (field.configured ? `已配置（${field.hint}），留空表示不修改` : "未配置") : field.placeholder} /></Form.Item>)}</Card>)}</Form></section>;
}
