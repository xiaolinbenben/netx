const overview = [
  { label: "在线节点", value: "0", detail: "暂无已连接节点" },
  { label: "本月流量", value: "0 GB", detail: "统计数据准备中" },
  { label: "服务状态", value: "正常", detail: "用户中心服务在线" },
];

export default function DashboardPage() {
  return (
    <main className="dashboard-shell">
      <header className="topbar">
        <a className="brand" href="/dashboard">
          <span className="brand-mark" aria-hidden="true">
            NX
          </span>
          <span>NetX</span>
        </a>
        <div className="topbar-meta">
          <span className="status-dot" aria-hidden="true" />
          服务正常
        </div>
      </header>

      <section className="hero">
        <div>
          <p className="eyebrow">NETX USER CENTER</p>
          <h1>网络连接，一目了然。</h1>
          <p className="hero-copy">欢迎进入 NetX 用户中心。节点、流量和服务配置将在这里集中管理。</p>
        </div>
        <div className="hero-code" aria-hidden="true">
          <span>ACCESS</span>
          <strong>READY</strong>
        </div>
      </section>

      <section className="overview-grid" aria-label="服务概览">
        {overview.map((item) => (
          <article className="metric" key={item.label}>
            <p>{item.label}</p>
            <strong>{item.value}</strong>
            <span>{item.detail}</span>
          </article>
        ))}
      </section>

      <section className="workspace-grid">
        <article className="panel panel-primary">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">WORKSPACE</p>
              <h2>开始使用 NetX</h2>
            </div>
            <span className="panel-index">01</span>
          </div>
          <p className="panel-copy">
            这是用户中心的基础版本。后续可以在这里接入账号、套餐、节点和配置管理功能。
          </p>
          <div className="next-step">
            <span className="next-step-icon" aria-hidden="true">+</span>
            <div>
              <strong>功能模块准备中</strong>
              <span>当前没有需要处理的事项</span>
            </div>
          </div>
        </article>

        <article className="panel system-panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">SYSTEM</p>
              <h2>系统信息</h2>
            </div>
            <span className="panel-index">02</span>
          </div>
          <dl className="system-list">
            <div>
              <dt>区域</dt>
              <dd>Global</dd>
            </div>
            <div>
              <dt>环境</dt>
              <dd>Production</dd>
            </div>
            <div>
              <dt>版本</dt>
              <dd>0.1.0</dd>
            </div>
          </dl>
        </article>
      </section>

      <footer className="footer">
        <span>NetX Enterprise Network Solution</span>
        <span>用户中心基础版本</span>
      </footer>
    </main>
  );
}
