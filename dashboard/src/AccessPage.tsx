import { useEffect, useState } from "react";

const noticeCountdownSeconds = 5;

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
  const [noticeCopied, setNoticeCopied] = useState(false);
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [noticeCountdown, setNoticeCountdown] = useState(noticeCountdownSeconds);
  const code = decodeURIComponent(
    window.location.pathname
      .replace(/^\/dashboard\/access\//, "")
      .replace(/^\/access\//, "")
      .replace(/\/$/, ""),
  );
  const currentURL = new URL(window.location.href);
  const showNotice = currentURL.searchParams.get("showNotice") === "1";
  const accessLink = `${currentURL.origin}${currentURL.pathname}`;
  const subscription = `${window.location.origin}/sub/${code}`;
  const [usage, setUsage] = useState<SubscriptionUsage | null>(null);
  const [usageError, setUsageError] = useState("");

  useEffect(() => {
    if (showNotice) {
      setNoticeOpen(true);
      setNoticeCountdown(noticeCountdownSeconds);
      return;
    }
    if (window.location.search) {
      window.history.replaceState(null, "", window.location.pathname);
    }
  }, [showNotice]);

  useEffect(() => {
    if (!noticeOpen) return;
    const timer = window.setInterval(() => {
      setNoticeCountdown((current) => {
        if (current <= 1) {
          window.clearInterval(timer);
          return 0;
        }
        return current - 1;
      });
    }, 1000);
    return () => window.clearInterval(timer);
  }, [noticeOpen]);

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
  async function copyAccessLink() { await navigator.clipboard.writeText(accessLink); setNoticeCopied(true); window.setTimeout(() => setNoticeCopied(false), 2200); }
  function confirmNotice() {
    if (noticeCountdown > 0) return;
    window.history.replaceState(null, "", window.location.pathname);
    setNoticeOpen(false);
  }

  const usagePercent = usage && usage.totalBytes > 0 ? Math.min(100, (usage.usedBytes / usage.totalBytes) * 100) : 0;
  return <main className="dashboard-shell access-shell">
    <header className="topbar"><a className="brand" href="/dashboard"><img className="brand-mark" src="/dashboard/netx-mark.png" alt="NetX" /><span>NetX</span></a><span className="access-state"><span className="status-dot" />专属订阅已激活</span></header>
    <section className="access-grid">
      <article className="access-card"><div className="card-label">SUBSCRIPTION URL</div><div className="url-box">{subscription}</div><button className="primary-button full-button" onClick={copy}>{copied ? "已复制订阅链接" : "复制 Clash 订阅链接"}<span>→</span></button><p className="access-instruction">复制订阅地址，将其添加到 Clash、Shadowrocket 或其他兼容客户端。</p></article>
      <article className="access-card"><div className="card-label">USAGE</div>{usage ? <><div className="usage-number">{formatTraffic(usage.usedBytes)}</div><div className="usage-bar"><span style={{ width: `${usagePercent}%` }} /></div><div className="usage-meta"><span>已用 {formatTraffic(usage.usedBytes)}</span><span>总量 {usage.unlimitedTotal ? "无限制" : formatTraffic(usage.totalBytes)}</span></div><dl className="access-list"><div><dt>剩余天数</dt><dd>{usage.unlimitedTime ? "∞" : `${usage.remainingDays} 天`}</dd></div><div><dt>到期时间</dt><dd>{formatExpiry(usage.expireAt, usage.unlimitedTime)}</dd></div><div><dt>节点状态</dt><dd className="online">在线</dd></div></dl></> : <div className="usage-loading">{usageError || "正在读取订阅用量..."}</div>}</article>
    </section>
    {noticeOpen && <div className="modal-backdrop notice-backdrop"><section className="notice-modal" role="dialog" aria-modal="true" aria-labelledby="usage-notice-title"><p className="eyebrow">USAGE NOTICE</p><h2 id="usage-notice-title">使用声明</h2><div className="notice-copy"><p>请在使用本网络节点前阅读并知晓以下声明：</p><ul><li>本网络服务仅供个人学习、技术研究及其他合法用途，请严格遵守国家法律法规及相关规定。</li><li>严禁利用本服务从事违法违规、侵害他人权益、网络攻击、服务滥用或传播违法、侵权内容等行为。</li><li>用户应自行承担自身使用产生的责任与后果；因用户违规或不当使用产生的责任，与服务提供者无关。</li><li>当前访问链接具有订阅查询及访问凭证属性，请妥善保管，勿分享或转发。</li></ul><div className="notice-link"><div className="notice-link-header"><span>请保存此访问链接</span><button className="notice-copy-button" type="button" onClick={copyAccessLink}>{noticeCopied ? "已复制" : "一键复制"}</button></div><code>{accessLink}</code></div></div><button className="primary-button full-button notice-confirm" disabled={noticeCountdown > 0} onClick={confirmNotice}>{noticeCountdown > 0 ? `请阅读使用声明（${noticeCountdown}秒）` : "我已阅读并且知晓使用声明"}<span>→</span></button></section></div>}
  </main>;
}
