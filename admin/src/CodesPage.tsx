import { useRef, useState } from "react";
import { App, Button, Form, Input, Modal, Select, Space, Tag, Typography } from "antd";
import { CopyOutlined, PlusOutlined } from "@ant-design/icons";
import { ProTable, type ActionType, type ProColumns } from "@ant-design/pro-components";
import { generateCodes, listCodes, type CodeStatus, type RedeemCode, updateCodeStatus } from "./api";
const statusMeta: Record<CodeStatus, { text: string; color: string }> = { unused: { text: "未使用", color: "green" }, reserved: { text: "待支付", color: "gold" }, used: { text: "已使用", color: "blue" }, void: { text: "已作废", color: "default" } };
const formatTime = (value: string) => value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "-";
export default function CodesPage() {
  const actionRef = useRef<ActionType | undefined>(undefined); const [open, setOpen] = useState(false); const [submitting, setSubmitting] = useState(false); const [form] = Form.useForm(); const { message, modal } = App.useApp();
  async function copy(value: string) { await navigator.clipboard.writeText(value); message.success("已复制"); }
  async function changeStatus(row: RedeemCode, status: CodeStatus) { modal.confirm({ title: status === "void" ? "确认作废兑换码？" : "确认恢复兑换码？", content: row.code, onOk: async () => { await updateCodeStatus(row.id, status); message.success("状态已更新"); actionRef.current?.reload(); } }); }
  async function submit() { setSubmitting(true); try { await generateCodes(await form.validateFields()); message.success("兑换码已生成"); setOpen(false); form.resetFields(); actionRef.current?.reload(); } catch (error) { if (error instanceof Error) message.error(error.message); } finally { setSubmitting(false); } }
  const columns: ProColumns<RedeemCode>[] = [
    { title: "兑换码", dataIndex: "code", copyable: true, width: 220, render: (_, row) => <Space><Typography.Text code>{row.code}</Typography.Text><Button type="link" size="small" icon={<CopyOutlined />} onClick={() => copy(row.code)} /></Space> },
    { title: "套餐", dataIndex: "plan", width: 120 },
    { title: "备注", dataIndex: "note", ellipsis: true, search: false, render: (_, row) => row.note || "-" },
    { title: "状态", dataIndex: "status", valueType: "select", valueEnum: Object.fromEntries(Object.entries(statusMeta).map(([key, value]) => [key, { text: value.text }])), render: (_, row) => <Tag color={statusMeta[row.status].color}>{statusMeta[row.status].text}</Tag> },
    { title: "创建时间", dataIndex: "createdAt", search: false, render: (_, row) => formatTime(row.createdAt) },
    { title: "使用时间", dataIndex: "usedAt", search: false, render: (_, row) => formatTime(row.usedAt) },
    { title: "操作", valueType: "option", width: 90, render: (_, row) => row.status === "unused" ? <Button type="link" danger onClick={() => changeStatus(row, "void")}>作废</Button> : row.status === "void" ? <Button type="link" onClick={() => changeStatus(row, "unused")}>恢复</Button> : null },
  ];
  return <section><div className="page-heading"><div><Typography.Title level={3}>兑换码</Typography.Title><Typography.Paragraph type="secondary">管理套餐库存、订阅地址和兑换码状态。</Typography.Paragraph></div><Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>生成兑换码</Button></div><ProTable<RedeemCode> actionRef={actionRef} rowKey="id" columns={columns} search={{ labelWidth: "auto" }} pagination={{ showSizeChanger: true, pageSize: 20 }} request={async (params) => { const data = await listCodes({ page: params.current || 1, size: params.pageSize || 20, ...(params.keyword ? { keyword: params.keyword } : {}), ...(params.status ? { status: params.status } : {}) }); return { data: data.items, total: data.total, success: true }; }} /><Modal title="生成兑换码" open={open} confirmLoading={submitting} onCancel={() => setOpen(false)} onOk={submit}><Form form={form} layout="vertical" initialValues={{ plan: "极速版" }}><Form.Item name="plan" label="套餐" rules={[{ required: true }]}><Select options={[{ label: "极速版", value: "极速版" }, { label: "至尊版", value: "至尊版" }]} /></Form.Item><Form.Item name="subscriptionUrl" label="3x-ui 订阅地址" rules={[{ required: true, message: "请填写订阅地址" }]}><Input placeholder="粘贴订阅链接" /></Form.Item><Form.Item name="note" label="备注"><Input maxLength={100} placeholder="可选" /></Form.Item></Form></Modal></section>;
}
