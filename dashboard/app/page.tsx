"use client";

import { useState } from "react";

const plans = [
  { id: "极速版", tag: "日常通用", price: "500", accent: "cyan", description: "稳定访问日常网站、办公和流媒体服务。", features: ["1000G / 年流量", "CN2 精品线路", "低至 160ms 延迟", "适合日常网络访问"] },
  { id: "至尊版", tag: "AI 专用优化", price: "800", accent: "gold", description: "为 ChatGPT、Claude、Gemini、Cursor、Codex 做专项适配。", features: ["1000G / 年流量", "CN2 精品线路", "低至 170ms 延迟", "AI 应用专项适配"] },
];

export default function DashboardPage() {
  const [redeemOpen, setRedeemOpen] = useState(false);
  const [code, setCode] = useState("");
  const [message, setMessage] = useState("");
  async function redeem() { const value = code.trim(); if (!value) { setMessage("请输入兑换码"); return; } try { const response = await fetch("/api/redeem", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ code: value }) }); const payload = await response.json(); if (!response.ok || !payload.data?.accessPath) throw new Error(payload.message || "兑换失败"); window.location.href = payload.data.accessPath; } catch (error) { setMessage(error instanceof Error ? error.message : "兑换失败，请检查卡密"); } }
  return <main className="dashboard-shell home-shell">
    <header className="topbar"><a className="brand" href="/dashboard"><span className="brand-mark">NX</span><span>NetX 网络服务中心</span></a></header>
    <section className="section-heading" id="plans"><div><p className="eyebrow">01 / PLANS</p><h2>选择你的 AI 专线</h2></div></section>
    <section className="plans-grid" aria-label="套餐选择">{plans.map((plan) => <article className={`plan-card ${plan.accent}`} key={plan.id}><div className="plan-top"><div><span className="plan-tag">{plan.tag}</span><h3>{plan.id}</h3></div><span className="plan-symbol">{plan.accent === "gold" ? "♛" : "ϟ"}</span></div><p className="plan-description">{plan.description}</p><div className="plan-price"><span>¥</span>{plan.price}<small>/ 年</small></div><ul>{plan.features.map((feature) => <li key={feature}><span>+</span>{feature}</li>)}</ul><button className="plan-button" onClick={() => setMessage(`${plan.id}订单已准备，支付宝支付配置完成后即可下单。`)}>购买 {plan.id}<span>→</span></button></article>)}</section>
    <section className="redeem-strip"><div><p className="eyebrow">02 / ACTIVATE</p><h2>已有卡密？立即兑换</h2><p>兑换后获得专属套餐。</p></div><button className="outline-button" onClick={() => setRedeemOpen(true)}>输入兑换码 <span>→</span></button></section>
    {message && <div className="toast" role="status">{message}<button onClick={() => setMessage("")}>×</button></div>}
    {redeemOpen && <div className="modal-backdrop" onClick={() => setRedeemOpen(false)}><div className="redeem-modal" onClick={(event) => event.stopPropagation()}><button className="modal-close" onClick={() => setRedeemOpen(false)}>×</button><p className="eyebrow">ACTIVATE ACCESS</p><h2>兑换你的 NetX 卡密</h2><p>输入后台生成的卡密，兑换专属订阅。</p><input autoFocus value={code} onChange={(event) => setCode(event.target.value)} onKeyDown={(event) => event.key === "Enter" && redeem()} placeholder="请输入 16 位小写字母或数字" /><button className="primary-button full-button" onClick={redeem}>继续兑换 <span>→</span></button>{message && <small className="error-text">{message}</small>}</div></div>}
  </main>;
}
