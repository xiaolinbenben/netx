import { useEffect, useState } from "react";

type SubscriptionUsage = {
  usedBytes: number;
  totalBytes: number;
  expireAt: number;
  remainingDays: number;
  unlimitedTotal: boolean;
  unlimitedTime: boolean;
};

function formatTraffic(bytes: number) {
  if (bytes <= 0) return "0 GB";
  const gigabytes = bytes / (1024 ** 3);
  return `${gigabytes >= 10 ? gigabytes.toFixed(0) : gigabytes.toFixed(2).replace(/\.00$/, "")} GB`;
}

function formatExpiry(expireAt: number, unlimited: boolean) {
  if (unlimited) return "无到期";
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(expireAt * 1000);
}

export default function AccessPage() {
  const [copied, setCopied] = useState(false);
  const code = decodeURIComponent(
    window.location.pathname
      .replace(/^\/dashboard\/access\//, "")
      .replace(/^\/access\//, "")
      .replace(/\/$/, ""),
  );
  const subscription = `${window.location.origin}/sub/${code}`;
  const [usage, setUsage] = useState<SubscriptionUsage | null>(null);
  const [usageError, setUsageError] = useState("");

  useEffect(() => {
    let active = true;
    fetch(`/api/subscription/${encodeURIComponent(code)}/usage`)
      .then(async (response) => {
        const payload = await response.json().catch(() => ({}));
        if (!response.ok || !payload.data) throw new Error(payload.message || "读取订阅用量失败");
        return payload.data as SubscriptionUsage;
      })
      .then((data) => { if (active) setUsage(data); })
      .catch((error) => { if (active) setUsageError(error instanceof Error ? error.message : "读取订阅用量失败"); });
    return () => { active = false; };
  }, [code]);

  async function copy() { await navigator.clipboard.writeText(subscription); setCopied(true); window.setTimeout(() => setCopied(false), 2200); }
  const usagePercent = usage && usage.totalBytes > 0 ? Math.min(100, (usage.usedBytes / usage.totalBytes) * 100) : 0;
  return <main className="dashboard-shell access-shell"><header className="topbar"><a className="brand" href="/dashboard"><img className="brand-mark" src="/dashboard/netx-mark.png" alt="NetX" /><span>NetX</span></a><span className="access-state"><span className="status-dot" />专属订阅已激活</span></header><section className="access-grid"><article className="access-card"><div className="card-label">SUBSCRIPTION URL</div><div className="url-box">{subscription}</div><button className="primary-button full-button" onClick={copy}>{copied ? "已复制订阅链接" : "复制 Clash 订阅链接"}<span>→</span></button><p className="access-instruction">复制订阅地址，将其添加到 Clash、Shadowrocket 或其他兼容客户端。</p></article><article className="access-card"><div className="card-label">USAGE</div>{usage ? <><div className="usage-number">{formatTraffic(usage.usedBytes)}</div><div className="usage-bar"><span style={{ width: `${usagePercent}%` }} /></div><div className="usage-meta"><span>已用 {formatTraffic(usage.usedBytes)}</span><span>总量 {usage.unlimitedTotal ? "无限制" : formatTraffic(usage.totalBytes)}</span></div><dl className="access-list"><div><dt>剩余天数</dt><dd>{usage.unlimitedTime ? "∞" : `${usage.remainingDays} 天`}</dd></div><div><dt>到期时间</dt><dd>{formatExpiry(usage.expireAt, usage.unlimitedTime)}</dd></div><div><dt>节点状态</dt><dd className="online">在线</dd></div></dl></> : <div className="usage-loading">{usageError || "正在读取订阅用量..."}</div>}</article></section></main>;
}
