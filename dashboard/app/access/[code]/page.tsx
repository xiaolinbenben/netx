"use client";

import { useParams } from "next/navigation";
import { useState } from "react";

export default function AccessPage() {
  const params = useParams<{ code: string }>();
  const [copied, setCopied] = useState(false);
  const code = decodeURIComponent(params.code ?? "");
  const subscription = `https://netx.beisi.tech/sub/${code}`;
  async function copy() { await navigator.clipboard.writeText(subscription); setCopied(true); window.setTimeout(() => setCopied(false), 2200); }
  return <main className="dashboard-shell access-shell"><header className="topbar"><a className="brand" href="/dashboard"><span className="brand-mark">NX</span><span>NetX</span></a><span className="access-state"><span className="status-dot" />专属订阅已激活</span></header><section className="access-hero"><p className="eyebrow">NETX ACCESS / {code}</p><h1>你的专属网络已准备好。</h1><p>复制下方订阅地址，添加到 Clash、Shadowrocket 或其他兼容客户端。</p></section><section className="access-grid"><article className="access-card"><div className="card-label">SUBSCRIPTION URL</div><div className="url-box">{subscription}</div><button className="primary-button full-button" onClick={copy}>{copied ? "已复制订阅链接" : "复制 Clash 订阅链接"}<span>→</span></button></article><article className="access-card"><div className="card-label">USAGE</div><div className="usage-number">1000 <small>GB</small></div><div className="usage-bar"><span /></div><div className="usage-meta"><span>已使用 0 GB</span><span>剩余 1000 GB</span></div><dl className="access-list"><div><dt>套餐类型</dt><dd>卡密专线</dd></div><div><dt>有效期</dt><dd>兑换后 365 天</dd></div><div><dt>节点状态</dt><dd className="online">在线</dd></div></dl></article></section><p className="access-note">订阅链接已自动加入 NetX Clash 规则，OpenAI、Google 等服务将按专线策略路由。</p></main>;
}
