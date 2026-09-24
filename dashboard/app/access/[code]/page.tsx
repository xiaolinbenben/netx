"use client";

import { useParams } from "next/navigation";
import { useState } from "react";

export default function AccessPage() {
  const params = useParams<{ code: string }>();
  const [copied, setCopied] = useState(false);
  const code = decodeURIComponent(params.code ?? "");
  const origin = typeof window === "undefined" ? "https://netx.beisi.tech" : window.location.origin;
  const subscription = `${origin}/sub/${code}`;
  async function copy() { await navigator.clipboard.writeText(subscription); setCopied(true); window.setTimeout(() => setCopied(false), 2200); }
  return <main className="dashboard-shell access-shell"><header className="topbar"><a className="brand" href="/dashboard"><span className="brand-mark">NX</span><span>NetX</span></a><span className="access-state"><span className="status-dot" />专属订阅已激活</span></header><section className="access-grid"><article className="access-card"><div className="card-label">SUBSCRIPTION URL</div><div className="url-box">{subscription}</div><button className="primary-button full-button" onClick={copy}>{copied ? "已复制订阅链接" : "复制 Clash 订阅链接"}<span>→</span></button><p className="access-instruction">复制订阅地址，将其添加到 Clash、Shadowrocket 或其他兼容客户端。</p></article><article className="access-card"><div className="card-label">USAGE</div><div className="usage-number">1000 <small>GB</small></div><div className="usage-bar"><span /></div><div className="usage-meta"><span>已使用 0 GB</span><span>剩余 1000 GB</span></div><dl className="access-list"><div><dt>套餐类型</dt><dd>卡密专线</dd></div><div><dt>有效期</dt><dd>兑换后 365 天</dd></div><div><dt>节点状态</dt><dd className="online">在线</dd></div></dl></article></section></main>;
}
