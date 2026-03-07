import { useState, useEffect, useRef, useCallback } from "react";

// ─── DESIGN: Industrial-utilitarian control room aesthetic ───
// Inspired by mission control / NOC dashboards. Dense data, muted palette,
// high-contrast status signals. Monospace for data, geometric sans for UI.

const COLORS = {
  bg: "#0a0e14",
  surface: "#111820",
  surfaceHover: "#1a2230",
  border: "#1e2a3a",
  borderSubtle: "#151d28",
  text: "#c5cdd8",
  textMuted: "#5c6b7e",
  textBright: "#e8edf3",
  accent: "#3b82f6",
  accentMuted: "#1e3a5f",
  running: "#3b82f6",
  runningBg: "#0c1a2e",
  succeeded: "#10b981",
  succeededBg: "#0a1f18",
  failed: "#ef4444",
  failedBg: "#1f0a0a",
  paused: "#f59e0b",
  pausedBg: "#1f1a0a",
  pending: "#5c6b7e",
  skipped: "#5c6b7e",
  canceled: "#8b5cf6",
};

const STATUS_CONFIG = {
  running: { color: COLORS.running, bg: COLORS.runningBg, icon: "◉", label: "Running" },
  succeeded: { color: COLORS.succeeded, bg: COLORS.succeededBg, icon: "✓", label: "Succeeded" },
  failed: { color: COLORS.failed, bg: COLORS.failedBg, icon: "✗", label: "Failed" },
  paused: { color: COLORS.paused, bg: COLORS.pausedBg, icon: "⏸", label: "Paused" },
  pending: { color: COLORS.pending, bg: COLORS.bg, icon: "○", label: "Pending" },
  canceled: { color: COLORS.canceled, bg: COLORS.bg, icon: "⊘", label: "Canceled" },
  skipped: { color: COLORS.skipped, bg: COLORS.bg, icon: "—", label: "Skipped" },
};

// ─── MOCK DATA GENERATOR ───
const WORKFLOWS = ["order-processing", "payment-charge", "kyc-verification", "user-onboarding", "refund-process", "inventory-sync"];
const STEPS = {
  "order-processing": ["validate", "reserve-stock", "charge", "fulfill", "notify"],
  "payment-charge": ["validate-card", "fraud-check", "authorize", "capture"],
  "kyc-verification": ["fetch-docs", "face-detect", "face-match", "score"],
  "user-onboarding": ["create-account", "send-email", "setup-profile", "welcome-tour"],
  "refund-process": ["validate-request", "check-policy", "process-refund", "notify-customer"],
  "inventory-sync": ["fetch-sources", "reconcile", "update-db", "publish-events"],
};

const SUBSYSTEMS = [
  { id: "order-fulfillment", name: "Order Fulfillment", workflows: ["order-processing", "inventory-sync"], status: "healthy" },
  { id: "billing", name: "Billing", workflows: ["payment-charge", "refund-process"], status: "degraded" },
  { id: "identity", name: "Identity & KYC", workflows: ["kyc-verification", "user-onboarding"], status: "healthy" },
];

const MOCK_TRACE = {
  id: "4bf92f3577b34da6",
  spans: [
    { subsystem: "order-fulfillment", workflow: "order-processing", steps: ["validate", "reserve-stock", "confirm"], offset: 0, duration: 680, status: "succeeded", spanId: "s1", parentSpanId: "" },
    { subsystem: "billing", workflow: "payment-charge", steps: ["fraud-check", "authorize", "capture"], offset: 320, duration: 1100, status: "succeeded", spanId: "s2", parentSpanId: "s1" },
    { subsystem: "identity", workflow: "kyc-verification", steps: ["fetch-docs", "face-detect"], offset: 400, duration: 600, status: "succeeded", spanId: "s3", parentSpanId: "s1" },
    { subsystem: "order-fulfillment", workflow: "inventory-sync", steps: ["fetch-sources", "reconcile", "update-db"], offset: 1420, duration: 580, status: "succeeded", spanId: "s4", parentSpanId: "s2" },
    { subsystem: "billing", workflow: "refund-process", steps: ["validate-request", "notify-customer"], offset: 1800, duration: 600, status: "failed", spanId: "s5", parentSpanId: "s2" },
  ],
  totalDuration: 2400,
};

const SUBSYSTEM_COLORS = { "order-fulfillment": "#3b82f6", "billing": "#a855f7", "identity": "#f59e0b" };

const randomId = () => Math.random().toString(36).substring(2, 8);
const randomChoice = (arr) => arr[Math.floor(Math.random() * arr.length)];
const randomInt = (min, max) => Math.floor(Math.random() * (max - min + 1)) + min;

function generateRun(statusOverride) {
  const wf = randomChoice(WORKFLOWS);
  const steps = STEPS[wf];
  const status = statusOverride || randomChoice(["running", "running", "succeeded", "succeeded", "succeeded", "failed", "paused"]);
  const currentStep = status === "succeeded" ? `✓ (${steps.length}/${steps.length})` :
    status === "failed" ? steps[randomInt(0, steps.length - 2)] :
    steps[randomInt(0, steps.length - 1)];
  const ago = status === "running" ? randomInt(1, 30) : randomInt(1, 300);
  return {
    id: randomId(),
    workflow: wf,
    status,
    currentStep,
    foreignId: `ord-${randomInt(1000, 9999)}`,
    startedAgo: ago,
    duration: status !== "running" ? randomInt(200, 15000) : null,
    retries: status === "failed" ? randomInt(1, 3) : 0,
    error: status === "failed" ? randomChoice(["insufficient_funds", "timeout", "rate_limited", "validation_error"]) : null,
  };
}

function generateStepExecutions(wf, runStatus) {
  const steps = STEPS[wf] || STEPS["order-processing"];
  let failIdx = runStatus === "failed" ? randomInt(1, steps.length - 1) : -1;
  return steps.map((name, i) => {
    let status = "succeeded";
    if (i === failIdx) status = "failed";
    else if (i > failIdx && failIdx >= 0) status = "skipped";
    else if (runStatus === "running" && i === steps.length - 1) status = "running";
    else if (runStatus === "running" && i >= steps.length - 2) status = randomChoice(["running", "pending"]);
    const duration = status === "succeeded" ? randomInt(50, 2000) : status === "failed" ? randomInt(100, 5000) : status === "running" ? randomInt(100, 800) : 0;
    return { name, status, duration, attempt: status === "failed" ? randomInt(2, 3) : 1, error: status === "failed" ? "Error: " + randomChoice(["timeout", "connection_refused", "invalid_response"]) : null };
  });
}

// ─── COMPONENTS ───

function StatusBadge({ status, size = "sm" }) {
  const cfg = STATUS_CONFIG[status] || STATUS_CONFIG.pending;
  const sz = size === "sm" ? { px: "6px 10px", fs: "11px" } : { px: "8px 14px", fs: "13px" };
  return (
    <span style={{
      display: "inline-flex", alignItems: "center", gap: 5,
      padding: sz.px, fontSize: sz.fs, fontWeight: 600, fontFamily: "'JetBrains Mono', monospace",
      color: cfg.color, background: cfg.bg, border: `1px solid ${cfg.color}22`,
      borderRadius: 4, letterSpacing: "0.5px", textTransform: "uppercase",
    }}>
      <span style={{ fontSize: size === "sm" ? 10 : 13 }}>{cfg.icon}</span>
      {cfg.label}
    </span>
  );
}

function StatsCard({ label, value, trend, color, onClick }) {
  return (
    <button onClick={onClick} style={{
      flex: 1, padding: "16px 20px", background: COLORS.surface, border: `1px solid ${COLORS.border}`,
      borderRadius: 8, cursor: "pointer", textAlign: "left", transition: "all 0.15s",
      borderBottom: `3px solid ${color}`,
    }}
      onMouseEnter={e => { e.currentTarget.style.background = COLORS.surfaceHover; e.currentTarget.style.transform = "translateY(-1px)"; }}
      onMouseLeave={e => { e.currentTarget.style.background = COLORS.surface; e.currentTarget.style.transform = "none"; }}
    >
      <div style={{ fontSize: 11, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 1.2, fontWeight: 600, marginBottom: 8 }}>{label}</div>
      <div style={{ display: "flex", alignItems: "baseline", gap: 10 }}>
        <span style={{ fontSize: 32, fontWeight: 700, color: COLORS.textBright, fontFamily: "'JetBrains Mono', monospace" }}>{value}</span>
        {trend && <span style={{ fontSize: 12, color: trend.startsWith("↑") ? COLORS.succeeded : trend.startsWith("↓") ? COLORS.failed : COLORS.textMuted, fontWeight: 600 }}>{trend}</span>}
      </div>
    </button>
  );
}

function LiveTicker({ runs }) {
  return (
    <div style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 8, overflow: "hidden" }}>
      <div style={{ padding: "12px 16px", borderBottom: `1px solid ${COLORS.border}`, display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ width: 8, height: 8, borderRadius: "50%", background: COLORS.succeeded, animation: "pulse 2s infinite" }} />
          <span style={{ fontSize: 12, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 1 }}>Live Feed</span>
        </div>
        <span style={{ fontSize: 11, color: COLORS.textMuted }}>Streaming</span>
      </div>
      <div style={{ maxHeight: 280, overflow: "auto" }}>
        {runs.slice(0, 8).map((r, i) => (
          <div key={r.id + i} style={{
            padding: "10px 16px", borderBottom: `1px solid ${COLORS.borderSubtle}`,
            display: "grid", gridTemplateColumns: "24px 1fr 140px 80px 90px", alignItems: "center", gap: 8,
            fontSize: 13, animation: i === 0 ? "slideIn 0.3s ease" : "none",
            background: i === 0 ? `${COLORS.accent}08` : "transparent",
          }}>
            <span style={{ color: STATUS_CONFIG[r.status]?.color, fontSize: 14 }}>{STATUS_CONFIG[r.status]?.icon}</span>
            <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, color: COLORS.textMuted }}>{r.id}</span>
            <span style={{ color: COLORS.text, fontWeight: 500, fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{r.workflow}</span>
            <span style={{ color: COLORS.textMuted, fontSize: 11 }}>{r.currentStep?.substring(0, 12)}</span>
            <span style={{ color: COLORS.textMuted, fontSize: 11, textAlign: "right" }}>{r.startedAgo}s ago</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function RunRow({ run, selected, onClick }) {
  const cfg = STATUS_CONFIG[run.status] || STATUS_CONFIG.pending;
  return (
    <div onClick={() => onClick(run)} style={{
      display: "grid", gridTemplateColumns: "100px 70px 160px 160px 80px 70px",
      padding: "12px 16px", borderBottom: `1px solid ${COLORS.borderSubtle}`, cursor: "pointer",
      background: selected ? COLORS.surfaceHover : "transparent",
      borderLeft: selected ? `3px solid ${COLORS.accent}` : "3px solid transparent",
      transition: "all 0.1s",
    }}
      onMouseEnter={e => { if (!selected) e.currentTarget.style.background = `${COLORS.surfaceHover}80`; }}
      onMouseLeave={e => { if (!selected) e.currentTarget.style.background = "transparent"; }}
    >
      <div><StatusBadge status={run.status} /></div>
      <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, color: COLORS.accent }}>{run.id}</div>
      <div style={{ fontSize: 13, color: COLORS.text, fontWeight: 500 }}>{run.workflow}</div>
      <div style={{ fontSize: 12, color: COLORS.textMuted, display: "flex", alignItems: "center", gap: 4 }}>
        {run.status === "failed" && <span style={{ color: COLORS.failed }}>↳</span>}
        {run.currentStep}
        {run.retries > 0 && <span style={{ fontSize: 10, color: COLORS.failed }}>(retry {run.retries})</span>}
      </div>
      <div style={{ fontSize: 12, color: COLORS.textMuted }}>{run.startedAgo}s ago</div>
      <div style={{ fontSize: 12, fontFamily: "'JetBrains Mono', monospace", color: COLORS.textMuted }}>{run.duration ? `${(run.duration / 1000).toFixed(1)}s` : "—"}</div>
    </div>
  );
}

function TimelineBar({ step, maxDuration }) {
  const cfg = STATUS_CONFIG[step.status] || STATUS_CONFIG.pending;
  const pct = maxDuration > 0 ? (step.duration / maxDuration) * 100 : 0;
  const offset = step._offset || 0;
  return (
    <div style={{ display: "grid", gridTemplateColumns: "130px 1fr 60px 80px", alignItems: "center", gap: 12, padding: "6px 0" }}>
      <span style={{ fontSize: 12, fontFamily: "'JetBrains Mono', monospace", color: COLORS.text, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{step.name}</span>
      <div style={{ position: "relative", height: 22, background: `${COLORS.border}40`, borderRadius: 3 }}>
        <div style={{
          position: "absolute", left: `${offset}%`, width: `${Math.max(pct, 2)}%`, height: "100%",
          background: step.status === "failed" ? `repeating-linear-gradient(90deg, ${cfg.color}, ${cfg.color} 4px, ${cfg.color}80 4px, ${cfg.color}80 8px)` :
            step.status === "running" ? `linear-gradient(90deg, ${cfg.color}, ${cfg.color}80)` : cfg.color,
          borderRadius: 3, transition: "width 0.3s",
          animation: step.status === "running" ? "barPulse 1.5s infinite" : "none",
        }} />
        {step.attempt > 1 && (
          <div style={{ position: "absolute", right: 4, top: 3, fontSize: 10, color: "#fff", fontWeight: 700 }}>×{step.attempt}</div>
        )}
      </div>
      <span style={{ fontSize: 11, fontFamily: "'JetBrains Mono', monospace", color: COLORS.textMuted, textAlign: "right" }}>{step.duration > 0 ? `${step.duration}ms` : "—"}</span>
      <div style={{ display: "flex", justifyContent: "flex-end" }}><StatusBadge status={step.status} /></div>
    </div>
  );
}

function RunDetail({ run, onClose }) {
  const [view, setView] = useState("timeline");
  const steps = generateStepExecutions(run.workflow, run.status);
  const maxDur = Math.max(...steps.map(s => s.duration), 1);
  let cumulOffset = 0;
  const stepsWithOffset = steps.map(s => {
    const o = { ...s, _offset: (cumulOffset / (maxDur * 1.5)) * 100 };
    if (s.status === "succeeded" || s.status === "failed") cumulOffset += s.duration * 0.3;
    return o;
  });

  return (
    <div style={{
      position: "fixed", top: 0, right: 0, width: "55%", height: "100vh",
      background: COLORS.bg, borderLeft: `1px solid ${COLORS.border}`,
      boxShadow: "-8px 0 40px rgba(0,0,0,0.5)", zIndex: 100,
      display: "flex", flexDirection: "column", animation: "slideInRight 0.2s ease",
    }}>
      <div style={{ padding: "20px 24px", borderBottom: `1px solid ${COLORS.border}`, flexShrink: 0 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <button onClick={onClose} style={{ background: "none", border: "none", color: COLORS.textMuted, cursor: "pointer", fontSize: 18, padding: 4 }}>←</button>
            <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 14, color: COLORS.accent }}>{run.id}</span>
            <StatusBadge status={run.status} size="md" />
          </div>
          <span style={{ fontSize: 12, color: COLORS.textMuted }}>{run.workflow}</span>
        </div>

        <div style={{ display: "flex", gap: 24, fontSize: 12, color: COLORS.textMuted, marginBottom: 16 }}>
          <span>Foreign ID: <span style={{ color: COLORS.text, fontFamily: "'JetBrains Mono', monospace" }}>{run.foreignId}</span></span>
          <span>Duration: <span style={{ color: COLORS.text }}>{run.duration ? `${(run.duration / 1000).toFixed(1)}s` : "in progress"}</span></span>
          <span>Started: <span style={{ color: COLORS.text }}>{run.startedAgo}s ago</span></span>
        </div>

        <div style={{ display: "flex", gap: 8 }}>
          {["Retry", "Retry from Failed", "Cancel", "Skip"].map(action => (
            <button key={action} style={{
              padding: "6px 14px", fontSize: 12, fontWeight: 600, borderRadius: 5, cursor: "pointer",
              background: action === "Retry" ? COLORS.accent : "transparent",
              color: action === "Retry" ? "#fff" : action === "Cancel" ? COLORS.failed : COLORS.textMuted,
              border: `1px solid ${action === "Retry" ? COLORS.accent : COLORS.border}`,
              transition: "all 0.15s",
            }}
              onMouseEnter={e => { if (action !== "Retry") e.currentTarget.style.background = COLORS.surfaceHover; }}
              onMouseLeave={e => { if (action !== "Retry") e.currentTarget.style.background = "transparent"; }}
            >{action}</button>
          ))}
        </div>
      </div>

      <div style={{ padding: "0 24px", borderBottom: `1px solid ${COLORS.border}`, display: "flex", gap: 0, flexShrink: 0 }}>
        {["timeline", "steps", "graph"].map(v => (
          <button key={v} onClick={() => setView(v)} style={{
            padding: "12px 20px", fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: 0.8,
            background: "none", border: "none", cursor: "pointer",
            color: view === v ? COLORS.accent : COLORS.textMuted,
            borderBottom: view === v ? `2px solid ${COLORS.accent}` : "2px solid transparent",
          }}>{v}</button>
        ))}
      </div>

      <div style={{ flex: 1, overflow: "auto", padding: 24 }}>
        {view === "timeline" && (
          <div>
            <div style={{ fontSize: 11, color: COLORS.textMuted, marginBottom: 16, display: "flex", justifyContent: "space-between" }}>
              <span>STEP TIMELINE</span>
              <span>0ms — {maxDur}ms</span>
            </div>
            {stepsWithOffset.map((s, i) => <TimelineBar key={i} step={s} maxDuration={maxDur} />)}
          </div>
        )}
        {view === "steps" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {steps.map((s, i) => (
              <div key={i} style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 6, padding: 16 }}>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                  <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 13, color: COLORS.text }}>{s.name}</span>
                  <StatusBadge status={s.status} />
                </div>
                <div style={{ display: "flex", gap: 20, fontSize: 11, color: COLORS.textMuted }}>
                  <span>Duration: {s.duration}ms</span>
                  <span>Attempt: {s.attempt}</span>
                </div>
                {s.error && (
                  <div style={{ marginTop: 8, padding: "8px 12px", background: COLORS.failedBg, border: `1px solid ${COLORS.failed}22`, borderRadius: 4, fontSize: 12, fontFamily: "'JetBrains Mono', monospace", color: COLORS.failed }}>{s.error}</div>
                )}
              </div>
            ))}
          </div>
        )}
        {view === "graph" && (
          <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 0, paddingTop: 16 }}>
            {steps.map((s, i) => {
              const cfg = STATUS_CONFIG[s.status];
              return (
                <div key={i} style={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
                  <div style={{
                    width: 180, padding: "12px 16px", background: cfg.bg, border: `2px solid ${cfg.color}`,
                    borderRadius: 8, textAlign: "center",
                    animation: s.status === "running" ? "nodePulse 2s infinite" : "none",
                  }}>
                    <div style={{ fontSize: 12, fontFamily: "'JetBrains Mono', monospace", color: cfg.color, fontWeight: 600 }}>{s.name}</div>
                    <div style={{ fontSize: 10, color: COLORS.textMuted, marginTop: 4 }}>{s.duration}ms · {cfg.label}</div>
                  </div>
                  {i < steps.length - 1 && (
                    <div style={{ width: 2, height: 24, background: COLORS.border }} />
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── STEP DURATION & LATENCY CARD ───

const STEP_DURATION_DATA = [
  { name: "charge-card", queue: { p50: 45, p95: 120, p99: 340 }, exec: { p50: 295, p95: 1080, p99: 3460 }, total: { p50: 340, p95: 1200, p99: 3800 }, bottleneck: true, samples: 4821,
    trend: [310, 325, 340, 380, 520, 410, 340] },
  { name: "face-match", queue: { p50: 30, p95: 80, p99: 200 }, exec: { p50: 190, p95: 500, p99: 900 }, total: { p50: 220, p95: 580, p99: 1100 }, bottleneck: false, samples: 2103,
    trend: [200, 210, 215, 220, 225, 218, 220] },
  { name: "reconcile", queue: { p50: 20, p95: 55, p99: 140 }, exec: { p50: 130, p95: 290, p99: 520 }, total: { p50: 150, p95: 345, p99: 660 }, bottleneck: false, samples: 8430,
    trend: [140, 142, 148, 150, 155, 152, 150] },
  { name: "validate", queue: { p50: 5, p95: 15, p99: 40 }, exec: { p50: 40, p95: 75, p99: 140 }, total: { p50: 45, p95: 90, p99: 180 }, bottleneck: false, samples: 12304,
    trend: [44, 45, 43, 46, 45, 44, 45] },
  { name: "notify", queue: { p50: 8, p95: 25, p99: 60 }, exec: { p50: 22, p95: 35, p99: 60 }, total: { p50: 30, p95: 60, p99: 120 }, bottleneck: false, samples: 11890,
    trend: [28, 29, 30, 31, 30, 29, 30] },
];

const DAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

function MiniSparkline({ data, width = 80, height = 24, color = COLORS.accent }) {
  const max = Math.max(...data);
  const min = Math.min(...data);
  const range = max - min || 1;
  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * width;
    const y = height - ((v - min) / range) * (height - 4) - 2;
    return `${x},${y}`;
  }).join(" ");

  // Determine trend direction for color
  const trendUp = data[data.length - 1] > data[0] * 1.1;
  const trendColor = trendUp ? COLORS.failed : color;

  return (
    <svg width={width} height={height} style={{ display: "block" }}>
      <polyline points={points} fill="none" stroke={trendColor} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={(data.length - 1) / (data.length - 1) * width} cy={height - ((data[data.length - 1] - min) / range) * (height - 4) - 2} r="2.5" fill={trendColor} />
    </svg>
  );
}

function DurationBar({ queue, exec, maxTotal }) {
  const total = queue + exec;
  const pct = (total / maxTotal) * 100;
  const queuePct = total > 0 ? (queue / total) * 100 : 0;
  const queueWarning = queuePct > 50;

  return (
    <div style={{ position: "relative", height: 18, background: `${COLORS.border}30`, borderRadius: 3, overflow: "hidden", cursor: "pointer" }}
      title={`Queue: ${queue}ms (${Math.round(queuePct)}%) · Exec: ${exec}ms (${Math.round(100 - queuePct)}%)`}
    >
      <div style={{ position: "absolute", left: 0, top: 0, height: "100%", width: `${pct}%`, display: "flex", borderRadius: 3, overflow: "hidden" }}>
        <div style={{
          width: `${queuePct}%`, height: "100%",
          background: queueWarning
            ? `repeating-linear-gradient(90deg, ${COLORS.paused}60, ${COLORS.paused}60 3px, ${COLORS.paused}30 3px, ${COLORS.paused}30 6px)`
            : `repeating-linear-gradient(90deg, ${COLORS.accent}40, ${COLORS.accent}40 3px, ${COLORS.accent}20 3px, ${COLORS.accent}20 6px)`,
        }} />
        <div style={{ flex: 1, height: "100%", background: COLORS.accent }} />
      </div>
      {queueWarning && (
        <span style={{ position: "absolute", right: 4, top: 1, fontSize: 10, color: COLORS.paused }}>⚠</span>
      )}
    </div>
  );
}

function PercentileCell({ value, thresholds = [500, 2000] }) {
  const color = value < thresholds[0] ? COLORS.succeeded : value < thresholds[1] ? COLORS.paused : COLORS.failed;
  return (
    <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 11, color, fontWeight: 500 }}>
      {value >= 1000 ? `${(value / 1000).toFixed(1)}s` : `${value}ms`}
    </span>
  );
}

function StepDurationCard() {
  const [scope, setScope] = useState("all");
  const [selectedStep, setSelectedStep] = useState(null);
  const [sortBy, setSortBy] = useState("p95");

  const sorted = [...STEP_DURATION_DATA].sort((a, b) => {
    if (a.bottleneck && !b.bottleneck) return -1;
    if (!a.bottleneck && b.bottleneck) return 1;
    return b.total[sortBy] - a.total[sortBy];
  });

  const maxTotal = Math.max(...sorted.map(s => s.total.p50));

  return (
    <div style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 8, padding: 24, marginBottom: 16 }}>
      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 20 }}>
        <div style={{ fontSize: 12, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 1 }}>
          Step Duration & Latency
        </div>
        <div style={{ display: "flex", gap: 6 }}>
          {["all", "single"].map(s => (
            <button key={s} onClick={() => setScope(s)} style={{
              padding: "4px 12px", fontSize: 11, fontWeight: 600, borderRadius: 4, cursor: "pointer",
              background: scope === s ? COLORS.accentMuted : "transparent",
              color: scope === s ? COLORS.accent : COLORS.textMuted,
              border: `1px solid ${scope === s ? COLORS.accent + "40" : COLORS.border}`,
            }}>{s === "all" ? "All Workflows" : "Single ▾"}</button>
          ))}
        </div>
      </div>

      {/* Table header */}
      <div style={{
        display: "grid", gridTemplateColumns: "140px 1fr 60px 60px 60px 86px",
        gap: 12, padding: "8px 0", borderBottom: `1px solid ${COLORS.border}`,
        fontSize: 10, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 0.8,
      }}>
        <span>Step</span>
        <span>Queue + Exec (p50)</span>
        {["p50", "p95", "p99"].map(p => (
          <span key={p} onClick={() => setSortBy(p)} style={{
            cursor: "pointer", textAlign: "right",
            color: sortBy === p ? COLORS.accent : COLORS.textMuted,
            textDecoration: sortBy === p ? "underline" : "none",
          }}>{p} {sortBy === p ? "↓" : ""}</span>
        ))}
        <span style={{ textAlign: "center" }}>7d Trend</span>
      </div>

      {/* Step rows */}
      {sorted.map((step) => (
        <div key={step.name}>
          <div
            onClick={() => setSelectedStep(selectedStep === step.name ? null : step.name)}
            style={{
              display: "grid", gridTemplateColumns: "140px 1fr 60px 60px 60px 86px",
              gap: 12, padding: "10px 0", borderBottom: `1px solid ${COLORS.borderSubtle}`,
              cursor: "pointer", alignItems: "center",
              background: selectedStep === step.name ? `${COLORS.accent}08` : "transparent",
            }}
            onMouseEnter={e => { if (selectedStep !== step.name) e.currentTarget.style.background = `${COLORS.surfaceHover}60`; }}
            onMouseLeave={e => { if (selectedStep !== step.name) e.currentTarget.style.background = "transparent"; }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              {step.bottleneck && <span style={{ fontSize: 10 }} title="Bottleneck: p95 > 2× median">🔴</span>}
              <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, color: COLORS.text, fontWeight: step.bottleneck ? 600 : 400 }}>{step.name}</span>
            </div>
            <DurationBar queue={step.queue.p50} exec={step.exec.p50} maxTotal={maxTotal} />
            <div style={{ textAlign: "right" }}><PercentileCell value={step.total.p50} /></div>
            <div style={{ textAlign: "right" }}><PercentileCell value={step.total.p95} /></div>
            <div style={{ textAlign: "right" }}><PercentileCell value={step.total.p99} /></div>
            <div style={{ display: "flex", justifyContent: "center" }}>
              <MiniSparkline data={step.trend} />
            </div>
          </div>

          {/* Expanded detail: trend chart */}
          {selectedStep === step.name && (
            <div style={{
              padding: "16px 0 16px 20px", borderBottom: `1px solid ${COLORS.border}`,
              background: `${COLORS.accent}04`, animation: "slideIn 0.15s ease",
            }}>
              <div style={{ fontSize: 11, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 0.8, marginBottom: 12 }}>
                {step.name} — 7-Day Trend · {step.samples.toLocaleString()} samples
              </div>

              {/* Percentile breakdown row */}
              <div style={{ display: "flex", gap: 24, marginBottom: 16 }}>
                {[
                  { label: "Queue Wait", data: step.queue, color: COLORS.paused },
                  { label: "Execution", data: step.exec, color: COLORS.accent },
                  { label: "Total", data: step.total, color: COLORS.textBright },
                ].map(({ label, data, color }) => (
                  <div key={label} style={{ background: COLORS.bg, border: `1px solid ${COLORS.border}`, borderRadius: 6, padding: "10px 14px", minWidth: 140 }}>
                    <div style={{ fontSize: 10, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 0.6, marginBottom: 6, display: "flex", alignItems: "center", gap: 6 }}>
                      <span style={{ width: 8, height: 8, borderRadius: 2, background: label === "Queue Wait" ? `repeating-linear-gradient(90deg, ${color}60, ${color}60 2px, ${color}30 2px, ${color}30 4px)` : color, display: "inline-block" }} />
                      {label}
                    </div>
                    <div style={{ display: "flex", gap: 12, fontSize: 11, fontFamily: "'JetBrains Mono', monospace" }}>
                      <span style={{ color: COLORS.text }}>p50: <PercentileCell value={data.p50} /></span>
                      <span style={{ color: COLORS.text }}>p95: <PercentileCell value={data.p95} /></span>
                      <span style={{ color: COLORS.text }}>p99: <PercentileCell value={data.p99} /></span>
                    </div>
                  </div>
                ))}
              </div>

              {/* Trend chart visualization */}
              <div style={{ background: COLORS.bg, border: `1px solid ${COLORS.border}`, borderRadius: 6, padding: 16, position: "relative" }}>
                <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 8 }}>
                  <div style={{ display: "flex", gap: 16, fontSize: 10, color: COLORS.textMuted }}>
                    <span style={{ display: "flex", alignItems: "center", gap: 4 }}><span style={{ width: 12, height: 2, background: COLORS.accent, display: "inline-block" }} /> p50</span>
                    <span style={{ display: "flex", alignItems: "center", gap: 4 }}><span style={{ width: 12, height: 2, background: COLORS.paused, display: "inline-block", borderTop: "1px dashed" }} /> p95</span>
                    <span style={{ display: "flex", alignItems: "center", gap: 4 }}><span style={{ width: 12, height: 2, background: COLORS.failed, display: "inline-block", opacity: 0.5 }} /> p99</span>
                  </div>
                </div>
                <svg width="100%" height="120" viewBox="0 0 700 120" preserveAspectRatio="none" style={{ display: "block" }}>
                  {/* Grid lines */}
                  {[0, 1, 2, 3].map(i => (
                    <line key={i} x1="0" y1={i * 40} x2="700" y2={i * 40} stroke={COLORS.border} strokeWidth="0.5" />
                  ))}
                  {/* p99 area */}
                  <polyline points={step.trend.map((v, i) => {
                    const p99ratio = step.total.p99 / step.total.p50;
                    return `${i * 116.7},${120 - (v * p99ratio / (step.total.p99 * 1.2)) * 120}`;
                  }).join(" ")} fill="none" stroke={COLORS.failed} strokeWidth="1" opacity="0.4" strokeDasharray="4 3" />
                  {/* p95 line */}
                  <polyline points={step.trend.map((v, i) => {
                    const p95ratio = step.total.p95 / step.total.p50;
                    return `${i * 116.7},${120 - (v * p95ratio / (step.total.p99 * 1.2)) * 120}`;
                  }).join(" ")} fill="none" stroke={COLORS.paused} strokeWidth="1.5" strokeDasharray="6 3" />
                  {/* p50 line (main) */}
                  <polyline points={step.trend.map((v, i) => `${i * 116.7},${120 - (v / (step.total.p99 * 1.2)) * 120}`).join(" ")} fill="none" stroke={COLORS.accent} strokeWidth="2" />
                  {/* p50 dots */}
                  {step.trend.map((v, i) => (
                    <circle key={i} cx={i * 116.7} cy={120 - (v / (step.total.p99 * 1.2)) * 120} r="3" fill={COLORS.accent} stroke={COLORS.bg} strokeWidth="1.5" />
                  ))}
                </svg>
                <div style={{ display: "flex", justifyContent: "space-between", marginTop: 4 }}>
                  {DAYS.map(d => <span key={d} style={{ fontSize: 10, color: COLORS.textMuted }}>{d}</span>)}
                </div>
              </div>
            </div>
          )}
        </div>
      ))}

      {/* Legend */}
      <div style={{ display: "flex", gap: 16, marginTop: 12, fontSize: 10, color: COLORS.textMuted }}>
        <span style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <span style={{ width: 14, height: 8, borderRadius: 2, display: "inline-block", background: `repeating-linear-gradient(90deg, ${COLORS.accent}40, ${COLORS.accent}40 3px, ${COLORS.accent}20 3px, ${COLORS.accent}20 6px)` }} /> Queue wait
        </span>
        <span style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <span style={{ width: 14, height: 8, borderRadius: 2, display: "inline-block", background: COLORS.accent }} /> Execution
        </span>
        <span style={{ display: "flex", alignItems: "center", gap: 4 }}>
          🔴 Bottleneck (p95 > 2× median)
        </span>
        <span style={{ display: "flex", alignItems: "center", gap: 4 }}>
          ⚠ High queue ratio (&gt;50%)
        </span>
      </div>
    </div>
  );
}

// ─── SEARCH COMPONENTS ───

const MOCK_SEARCH_DATA = [
  { type: "run", id: "c4d7", workflow: "payment-charge", match: 'error:"insufficient_funds"', status: "failed", ago: "2m" },
  { type: "run", id: "a891", workflow: "payment-charge", match: 'error:"insufficient_funds"', status: "failed", ago: "18m" },
  { type: "run", id: "f234", workflow: "refund-process", match: 'error:"insufficient_funds"', status: "failed", ago: "1h" },
  { type: "step", id: "c4d7", step: "charge-card", match: "error: insufficient_funds (code 402)", status: "failed" },
  { type: "step", id: "a891", step: "charge-card", match: "error: insufficient_funds (code 402)", status: "failed" },
  { type: "log", id: "c4d7", step: "charge-card", match: 'ERR insufficient_funds for visa_8832, amount=149.99', status: "failed" },
  { type: "log", id: "a891", step: "charge-card", match: 'ERR insufficient_funds for mc_2201, amount=89.50', status: "failed" },
  { type: "log", id: "f234", step: "process-refund", match: 'WARN insufficient_funds reversal pending', status: "failed" },
];

const RECENT_SEARCHES = ["status:failed workflow:payment", "insufficient_funds", "ord-9382", "timeout AND step:charge"];

const QUERY_FIELDS = ["status:", "workflow:", "step:", "error:", "log:", "payload.", "started:", "duration:", "foreign_id:"];

function SearchPalette({ isOpen, onClose, onNavigate }) {
  const [query, setQuery] = useState("");
  const [selectedIdx, setSelectedIdx] = useState(0);
  const inputRef = useRef(null);

  useEffect(() => {
    if (isOpen && inputRef.current) inputRef.current.focus();
  }, [isOpen]);

  const results = query.length >= 2
    ? MOCK_SEARCH_DATA.filter(r => r.match.toLowerCase().includes(query.toLowerCase()) || r.workflow?.includes(query) || r.id?.includes(query))
    : [];

  const grouped = {
    runs: results.filter(r => r.type === "run"),
    steps: results.filter(r => r.type === "step"),
    logs: results.filter(r => r.type === "log"),
  };
  const flatResults = [...grouped.runs, ...grouped.steps, ...grouped.logs];

  const handleKeyDown = (e) => {
    if (e.key === "Escape") onClose();
    else if (e.key === "ArrowDown") { e.preventDefault(); setSelectedIdx(i => Math.min(i + 1, flatResults.length - 1)); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setSelectedIdx(i => Math.max(i - 1, 0)); }
    else if (e.key === "Enter" && flatResults[selectedIdx]) { onNavigate(flatResults[selectedIdx]); onClose(); }
    else if (e.key === "Tab") { e.preventDefault(); onClose(); onNavigate({ type: "advanced", query }); }
  };

  const highlightMatch = (text, q) => {
    if (!q || q.length < 2) return text;
    const idx = text.toLowerCase().indexOf(q.toLowerCase());
    if (idx === -1) return text;
    return (
      <span>{text.substring(0, idx)}<span style={{ background: `${COLORS.accent}30`, color: COLORS.accent, borderRadius: 2, padding: "0 2px" }}>{text.substring(idx, idx + q.length)}</span>{text.substring(idx + q.length)}</span>
    );
  };

  if (!isOpen) return null;

  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 200, display: "flex", justifyContent: "center", paddingTop: 80 }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.6)", backdropFilter: "blur(4px)" }} />
      <div style={{
        position: "relative", width: 620, maxHeight: "70vh", background: COLORS.surface,
        border: `1px solid ${COLORS.border}`, borderRadius: 12, overflow: "hidden",
        boxShadow: "0 20px 60px rgba(0,0,0,0.5)", animation: "slideIn 0.15s ease",
        display: "flex", flexDirection: "column",
      }}>
        {/* Search Input */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "14px 18px", borderBottom: `1px solid ${COLORS.border}` }}>
          <span style={{ color: COLORS.textMuted, fontSize: 16 }}>🔍</span>
          <input ref={inputRef} value={query} onChange={e => { setQuery(e.target.value); setSelectedIdx(0); }}
            onKeyDown={handleKeyDown}
            placeholder="Search runs, steps, errors, payloads, logs..."
            style={{
              flex: 1, background: "none", border: "none", outline: "none",
              color: COLORS.textBright, fontSize: 15, fontFamily: "'Geist', sans-serif",
            }}
          />
          <span style={{ fontSize: 11, color: COLORS.textMuted, padding: "3px 6px", background: COLORS.bg, borderRadius: 4, fontFamily: "'JetBrains Mono', monospace" }}>⌘K</span>
        </div>

        {/* Results / Recent */}
        <div style={{ flex: 1, overflow: "auto", padding: "8px 0" }}>
          {query.length < 2 ? (
            <div style={{ padding: "8px 18px" }}>
              <div style={{ fontSize: 11, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 1, marginBottom: 8 }}>Recent Searches</div>
              {RECENT_SEARCHES.map((s, i) => (
                <div key={i} onClick={() => setQuery(s)} style={{
                  padding: "8px 12px", borderRadius: 6, cursor: "pointer", display: "flex", alignItems: "center", gap: 10,
                  fontSize: 13, color: COLORS.text, fontFamily: "'JetBrains Mono', monospace",
                }}
                  onMouseEnter={e => e.currentTarget.style.background = COLORS.surfaceHover}
                  onMouseLeave={e => e.currentTarget.style.background = "transparent"}
                >
                  <span style={{ color: COLORS.textMuted, fontSize: 12 }}>↻</span>
                  {s}
                </div>
              ))}
              <div style={{ marginTop: 16, fontSize: 11, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 1, marginBottom: 8 }}>Query Syntax</div>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
                {QUERY_FIELDS.map(f => (
                  <span key={f} onClick={() => setQuery(f)} style={{
                    padding: "3px 8px", fontSize: 11, fontFamily: "'JetBrains Mono', monospace",
                    background: COLORS.bg, border: `1px solid ${COLORS.border}`, borderRadius: 4,
                    color: COLORS.accent, cursor: "pointer",
                  }}>{f}</span>
                ))}
              </div>
            </div>
          ) : flatResults.length === 0 ? (
            <div style={{ padding: "24px 18px", textAlign: "center", color: COLORS.textMuted, fontSize: 13 }}>
              No results for "{query}"
            </div>
          ) : (
            <div>
              {Object.entries(grouped).filter(([, items]) => items.length > 0).map(([group, items]) => (
                <div key={group}>
                  <div style={{ padding: "8px 18px 4px", fontSize: 11, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 1 }}>
                    {group} ({items.length})
                  </div>
                  {items.map((item, i) => {
                    const globalIdx = flatResults.indexOf(item);
                    const isSelected = globalIdx === selectedIdx;
                    return (
                      <div key={`${item.id}-${item.type}-${i}`}
                        onClick={() => { onNavigate(item); onClose(); }}
                        style={{
                          padding: "8px 18px", display: "grid",
                          gridTemplateColumns: group === "log" ? "20px 50px 100px 1fr" : "20px 50px 140px 1fr 50px",
                          alignItems: "center", gap: 8, cursor: "pointer", fontSize: 12,
                          background: isSelected ? COLORS.accentMuted : "transparent",
                          borderLeft: isSelected ? `2px solid ${COLORS.accent}` : "2px solid transparent",
                        }}
                        onMouseEnter={e => { setSelectedIdx(globalIdx); e.currentTarget.style.background = COLORS.surfaceHover; }}
                        onMouseLeave={e => { if (!isSelected) e.currentTarget.style.background = "transparent"; }}
                      >
                        <span style={{ color: STATUS_CONFIG[item.status]?.color, fontSize: 13 }}>{STATUS_CONFIG[item.status]?.icon}</span>
                        <span style={{ fontFamily: "'JetBrains Mono', monospace", color: COLORS.accent, fontSize: 11 }}>{item.id}</span>
                        <span style={{ color: COLORS.text, fontSize: 11, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                          {item.type === "log" || item.type === "step" ? item.step : item.workflow}
                        </span>
                        <span style={{ fontFamily: "'JetBrains Mono', monospace", color: COLORS.textMuted, fontSize: 11, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                          {highlightMatch(item.match, query)}
                        </span>
                        {item.ago && <span style={{ fontSize: 10, color: COLORS.textMuted, textAlign: "right" }}>{item.ago}</span>}
                      </div>
                    );
                  })}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Footer */}
        <div style={{
          padding: "10px 18px", borderTop: `1px solid ${COLORS.border}`,
          display: "flex", justifyContent: "space-between", fontSize: 11, color: COLORS.textMuted,
        }}>
          <div style={{ display: "flex", gap: 16 }}>
            <span><kbd style={{ padding: "1px 5px", background: COLORS.bg, borderRadius: 3, border: `1px solid ${COLORS.border}`, fontFamily: "'JetBrains Mono', monospace", fontSize: 10 }}>↵</kbd> Open</span>
            <span><kbd style={{ padding: "1px 5px", background: COLORS.bg, borderRadius: 3, border: `1px solid ${COLORS.border}`, fontFamily: "'JetBrains Mono', monospace", fontSize: 10 }}>↑↓</kbd> Navigate</span>
            <span><kbd style={{ padding: "1px 5px", background: COLORS.bg, borderRadius: 3, border: `1px solid ${COLORS.border}`, fontFamily: "'JetBrains Mono', monospace", fontSize: 10 }}>⇥</kbd> Advanced</span>
            <span><kbd style={{ padding: "1px 5px", background: COLORS.bg, borderRadius: 3, border: `1px solid ${COLORS.border}`, fontFamily: "'JetBrains Mono', monospace", fontSize: 10 }}>esc</kbd> Close</span>
          </div>
          {flatResults.length > 0 && <span>{flatResults.length} results · 38ms</span>}
        </div>
      </div>
    </div>
  );
}

function AdvancedSearchBar({ searchQuery, setSearchQuery, queryMode, setQueryMode, resultCount }) {
  const [suggestions, setSuggestions] = useState([]);
  const inputRef = useRef(null);

  const handleInput = (val) => {
    setSearchQuery(val);
    // Show field suggestions when typing
    const lastToken = val.split(/\s+/).pop() || "";
    if (lastToken.length >= 1 && !lastToken.includes(":")) {
      setSuggestions(QUERY_FIELDS.filter(f => f.startsWith(lastToken)));
    } else {
      setSuggestions([]);
    }
  };

  const applySuggestion = (s) => {
    const tokens = searchQuery.split(/\s+/);
    tokens[tokens.length - 1] = s;
    setSearchQuery(tokens.join(" "));
    setSuggestions([]);
    inputRef.current?.focus();
  };

  // Syntax highlighting for raw query mode
  const highlightQuery = (q) => {
    return q.replace(/(status|workflow|step|error|log|payload\.\w+|started|duration|foreign_id):/g,
      '<span style="color:' + COLORS.accent + '">$&</span>')
      .replace(/\b(AND|OR|NOT)\b/g, '<span style="color:' + COLORS.paused + ';font-weight:700">$&</span>');
  };

  return (
    <div style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 8, padding: 16, marginBottom: 16 }}>
      {/* Search input row */}
      <div style={{ position: "relative", marginBottom: 12 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, background: COLORS.bg, border: `1px solid ${COLORS.border}`, borderRadius: 6, padding: "10px 14px" }}>
          <span style={{ color: COLORS.textMuted }}>🔍</span>
          {queryMode === "visual" ? (
            <input ref={inputRef} value={searchQuery} onChange={e => handleInput(e.target.value)}
              placeholder="Search across runs, steps, payloads, and logs..."
              style={{ flex: 1, background: "none", border: "none", outline: "none", color: COLORS.textBright, fontSize: 13, fontFamily: "'Geist', sans-serif" }}
            />
          ) : (
            <input ref={inputRef} value={searchQuery} onChange={e => handleInput(e.target.value)}
              placeholder='status:failed AND error:timeout AND started:last_24h'
              style={{ flex: 1, background: "none", border: "none", outline: "none", color: COLORS.textBright, fontSize: 13, fontFamily: "'JetBrains Mono', monospace" }}
            />
          )}
          <span style={{ fontSize: 11, color: COLORS.textMuted }}>{resultCount != null ? `${resultCount} results` : ""}</span>
        </div>
        {/* Auto-complete dropdown */}
        {suggestions.length > 0 && (
          <div style={{ position: "absolute", top: "100%", left: 0, right: 0, background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 6, marginTop: 4, zIndex: 10, overflow: "hidden" }}>
            {suggestions.map(s => (
              <div key={s} onClick={() => applySuggestion(s)} style={{
                padding: "8px 14px", cursor: "pointer", fontSize: 12, fontFamily: "'JetBrains Mono', monospace", color: COLORS.accent,
              }}
                onMouseEnter={e => e.currentTarget.style.background = COLORS.surfaceHover}
                onMouseLeave={e => e.currentTarget.style.background = "transparent"}
              >{s}</div>
            ))}
          </div>
        )}
      </div>

      {/* Mode toggle + facets row */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
          <span style={{ fontSize: 11, color: COLORS.textMuted, marginRight: 4 }}>Mode:</span>
          {["visual", "query"].map(m => (
            <button key={m} onClick={() => setQueryMode(m)} style={{
              padding: "4px 10px", fontSize: 11, fontWeight: 600, borderRadius: 4, cursor: "pointer",
              background: queryMode === m ? COLORS.accentMuted : "transparent",
              color: queryMode === m ? COLORS.accent : COLORS.textMuted,
              border: `1px solid ${queryMode === m ? COLORS.accent + "40" : COLORS.border}`,
              textTransform: "capitalize",
            }}>{m === "query" ? "⌨ Raw Query" : "◉ Visual"}</button>
          ))}
        </div>

        {searchQuery && (
          <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
            <span style={{
              display: "inline-flex", alignItems: "center", gap: 4, padding: "3px 10px",
              background: COLORS.accentMuted, border: `1px solid ${COLORS.accent}40`, borderRadius: 20,
              fontSize: 11, color: COLORS.accent, fontFamily: "'JetBrains Mono', monospace",
            }}>
              {searchQuery.length > 30 ? searchQuery.substring(0, 30) + "…" : searchQuery}
              <span onClick={() => setSearchQuery("")} style={{ cursor: "pointer", marginLeft: 4, opacity: 0.7 }}>×</span>
            </span>
            <button style={{
              padding: "4px 10px", fontSize: 11, fontWeight: 600, borderRadius: 4, cursor: "pointer",
              background: "transparent", color: COLORS.textMuted, border: `1px solid ${COLORS.border}`,
            }}>💾 Save View</button>
          </div>
        )}
      </div>
    </div>
  );
}

// ─── MAIN APP ───

export default function WorkflowMonitor() {
  const [page, setPage] = useState("dashboard");
  const [runs, setRuns] = useState(() => Array.from({ length: 40 }, () => generateRun()));
  const [selectedRun, setSelectedRun] = useState(null);
  const [filter, setFilter] = useState("all");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [queryMode, setQueryMode] = useState("visual");

  // Cmd+K handler
  useEffect(() => {
    const handler = (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setSearchOpen(true);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  // Simulate live updates
  useEffect(() => {
    const interval = setInterval(() => {
      setRuns(prev => {
        const newRun = generateRun("running");
        const updated = prev.map(r => {
          if (r.status === "running" && Math.random() > 0.85) {
            return { ...r, status: randomChoice(["succeeded", "succeeded", "failed"]), duration: randomInt(500, 8000) };
          }
          return { ...r, startedAgo: r.startedAgo + 3 };
        });
        return [newRun, ...updated].slice(0, 60);
      });
    }, 3000);
    return () => clearInterval(interval);
  }, []);

  const stats = {
    running: runs.filter(r => r.status === "running").length,
    succeeded: runs.filter(r => r.status === "succeeded").length,
    failed: runs.filter(r => r.status === "failed").length,
    paused: runs.filter(r => r.status === "paused").length,
  };
  const rate = Math.round(stats.succeeded / 5 * 60);

  const filteredRuns = filter === "all" ? runs : runs.filter(r => r.status === filter);

  const navItems = [
    { id: "dashboard", icon: "◫", label: "Dashboard" },
    { id: "subsystems", icon: "⬡", label: "Subsystems" },
    { id: "runs", icon: "▤", label: "Runs" },
    { id: "traces", icon: "⑆", label: "Traces" },
    { id: "workflows", icon: "◇", label: "Workflows" },
    { id: "analytics", icon: "◔", label: "Analytics" },
  ];

  return (
    <div style={{ display: "flex", height: "100vh", background: COLORS.bg, color: COLORS.text, fontFamily: "'Geist', 'SF Pro', -apple-system, sans-serif", overflow: "hidden" }}>
      <style>{`
        @import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&display=swap');
        @import url('https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&display=swap');
        * { box-sizing: border-box; margin: 0; padding: 0; }
        ::-webkit-scrollbar { width: 6px; }
        ::-webkit-scrollbar-track { background: ${COLORS.bg}; }
        ::-webkit-scrollbar-thumb { background: ${COLORS.border}; border-radius: 3px; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.4; } }
        @keyframes slideIn { from { opacity: 0; transform: translateY(-8px); } to { opacity: 1; transform: translateY(0); } }
        @keyframes slideInRight { from { transform: translateX(20px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
        @keyframes barPulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.7; } }
        @keyframes nodePulse { 0%, 100% { box-shadow: 0 0 0 0 ${COLORS.running}40; } 50% { box-shadow: 0 0 0 8px ${COLORS.running}00; } }
      `}</style>

      {/* Sidebar */}
      <nav style={{
        width: sidebarCollapsed ? 56 : 200, flexShrink: 0, background: COLORS.surface,
        borderRight: `1px solid ${COLORS.border}`, display: "flex", flexDirection: "column",
        transition: "width 0.2s", overflow: "hidden",
      }}>
        <div style={{ padding: sidebarCollapsed ? "16px 12px" : "16px 20px", borderBottom: `1px solid ${COLORS.border}`, display: "flex", alignItems: "center", gap: 10, cursor: "pointer" }}
          onClick={() => setSidebarCollapsed(!sidebarCollapsed)}>
          <span style={{ fontSize: 20, color: COLORS.accent }}>⬡</span>
          {!sidebarCollapsed && <span style={{ fontSize: 15, fontWeight: 700, color: COLORS.textBright, letterSpacing: -0.5 }}>FlowWatch</span>}
        </div>
        <div style={{ padding: "12px 8px", display: "flex", flexDirection: "column", gap: 2 }}>
          {/* Search trigger */}
          <button onClick={() => setSearchOpen(true)} style={{
            display: "flex", alignItems: "center", gap: 12,
            padding: sidebarCollapsed ? "10px 14px" : "10px 14px", borderRadius: 6,
            background: "transparent", color: COLORS.textMuted,
            border: `1px dashed ${COLORS.border}`, cursor: "pointer", fontSize: 13, fontWeight: 500,
            width: "100%", textAlign: "left", marginBottom: 8, transition: "all 0.1s",
          }}
            onMouseEnter={e => { e.currentTarget.style.borderColor = COLORS.accent; e.currentTarget.style.color = COLORS.accent; }}
            onMouseLeave={e => { e.currentTarget.style.borderColor = COLORS.border; e.currentTarget.style.color = COLORS.textMuted; }}
          >
            <span style={{ fontSize: 14, width: 20, textAlign: "center" }}>🔍</span>
            {!sidebarCollapsed && <span style={{ flex: 1 }}>Search</span>}
            {!sidebarCollapsed && <span style={{ fontSize: 10, padding: "2px 5px", background: COLORS.bg, borderRadius: 3, border: `1px solid ${COLORS.border}`, fontFamily: "'JetBrains Mono', monospace" }}>⌘K</span>}
          </button>

          {navItems.map(item => (
            <button key={item.id} onClick={() => setPage(item.id)} style={{
              display: "flex", alignItems: "center", gap: 12,
              padding: sidebarCollapsed ? "10px 14px" : "10px 14px", borderRadius: 6,
              background: page === item.id ? COLORS.accentMuted : "transparent",
              color: page === item.id ? COLORS.accent : COLORS.textMuted,
              border: "none", cursor: "pointer", fontSize: 13, fontWeight: 500,
              width: "100%", textAlign: "left", transition: "all 0.1s",
            }}
              onMouseEnter={e => { if (page !== item.id) e.currentTarget.style.background = COLORS.surfaceHover; }}
              onMouseLeave={e => { if (page !== item.id) e.currentTarget.style.background = "transparent"; }}
            >
              <span style={{ fontSize: 16, width: 20, textAlign: "center" }}>{item.icon}</span>
              {!sidebarCollapsed && item.label}
            </button>
          ))}
        </div>
      </nav>

      {/* Main Content */}
      <main style={{ flex: 1, overflow: "auto", padding: 24 }}>
        {page === "dashboard" && (
          <div style={{ maxWidth: 1200 }}>
            <h1 style={{ fontSize: 20, fontWeight: 700, color: COLORS.textBright, marginBottom: 24 }}>Dashboard</h1>
            <div style={{ display: "flex", gap: 12, marginBottom: 24 }}>
              <StatsCard label="Running" value={stats.running} trend="↑ 12%" color={COLORS.running} onClick={() => { setPage("runs"); setFilter("running"); }} />
              <StatsCard label="Succeeded" value={stats.succeeded} color={COLORS.succeeded} onClick={() => { setPage("runs"); setFilter("succeeded"); }} />
              <StatsCard label="Failed" value={stats.failed} trend={stats.failed > 3 ? "↑ spike" : "—"} color={COLORS.failed} onClick={() => { setPage("runs"); setFilter("failed"); }} />
              <StatsCard label="Paused" value={stats.paused} color={COLORS.paused} onClick={() => { setPage("runs"); setFilter("paused"); }} />
              <StatsCard label="Rate/min" value={rate} trend="↑ 8%" color={COLORS.accent} />
            </div>

            {/* Subsystem Health Cards */}
            <div style={{ marginBottom: 24 }}>
              <div style={{ fontSize: 11, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 1, marginBottom: 10 }}>Subsystem Health</div>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))", gap: 10 }}>
                {SUBSYSTEMS.map(ss => {
                  const ssRuns = runs.filter(r => ss.workflows.includes(r.workflow));
                  const failCount = ssRuns.filter(r => r.status === "failed").length;
                  const runCount = ssRuns.filter(r => r.status === "running").length;
                  const statusIcon = failCount > 3 ? "🔴" : failCount > 0 ? "🟡" : "🟢";
                  return (
                    <div key={ss.id} onClick={() => setPage("subsystems")} style={{
                      background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 6,
                      padding: "12px 16px", cursor: "pointer", borderLeft: `3px solid ${SUBSYSTEM_COLORS[ss.id] || COLORS.accent}`,
                      transition: "all 0.15s",
                    }}
                      onMouseEnter={e => e.currentTarget.style.background = COLORS.surfaceHover}
                      onMouseLeave={e => e.currentTarget.style.background = COLORS.surface}
                    >
                      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                        <span style={{ fontSize: 12, fontWeight: 600, color: COLORS.textBright }}>{ss.name}</span>
                        <span style={{ fontSize: 11 }}>{statusIcon}</span>
                      </div>
                      <div style={{ display: "flex", gap: 12, fontSize: 11 }}>
                        <span style={{ color: COLORS.running }}>◉ {runCount}</span>
                        <span style={{ color: failCount > 0 ? COLORS.failed : COLORS.textMuted }}>✗ {failCount}</span>
                        <span style={{ color: COLORS.textMuted }}>{ss.workflows.length} wfs</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
            <LiveTicker runs={runs} />
          </div>
        )}

        {/* ─── SUBSYSTEMS PAGE ─── */}
        {page === "subsystems" && (
          <div style={{ maxWidth: 1100 }}>
            <h1 style={{ fontSize: 20, fontWeight: 700, color: COLORS.textBright, marginBottom: 24 }}>Subsystems</h1>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))", gap: 12 }}>
              {SUBSYSTEMS.map(ss => {
                const ssRuns = runs.filter(r => ss.workflows.includes(r.workflow));
                const failCount = ssRuns.filter(r => r.status === "failed").length;
                const runningCount = ssRuns.filter(r => r.status === "running").length;
                const totalRecent = ssRuns.length || 1;
                const errRate = ((failCount / totalRecent) * 100).toFixed(1);
                const statusColor = failCount > 3 ? COLORS.failed : failCount > 0 ? COLORS.paused : COLORS.succeeded;
                const statusLabel = failCount > 3 ? "Degraded" : failCount > 0 ? "Warning" : "Healthy";
                const statusIcon = failCount > 3 ? "🔴" : failCount > 0 ? "🟡" : "🟢";
                return (
                  <div key={ss.id} style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 8, padding: 20, borderLeft: `4px solid ${SUBSYSTEM_COLORS[ss.id] || COLORS.accent}`, cursor: "pointer", transition: "all 0.15s" }}
                    onMouseEnter={e => e.currentTarget.style.borderColor = `${SUBSYSTEM_COLORS[ss.id] || COLORS.accent}60`}
                    onMouseLeave={e => e.currentTarget.style.borderColor = COLORS.border}
                    onClick={() => { setPage("runs"); setFilter("all"); }}
                  >
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                      <span style={{ fontSize: 15, fontWeight: 700, color: COLORS.textBright }}>{ss.name}</span>
                      <span style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12, color: statusColor, fontWeight: 600 }}>{statusIcon} {statusLabel}</span>
                    </div>
                    <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12, fontSize: 12 }}>
                      <div><span style={{ color: COLORS.textMuted, display: "block", fontSize: 10, textTransform: "uppercase", letterSpacing: 0.8, marginBottom: 4 }}>Running</span><span style={{ color: COLORS.running, fontFamily: "'JetBrains Mono', monospace", fontWeight: 600 }}>{runningCount}</span></div>
                      <div><span style={{ color: COLORS.textMuted, display: "block", fontSize: 10, textTransform: "uppercase", letterSpacing: 0.8, marginBottom: 4 }}>Failed/1h</span><span style={{ color: failCount > 0 ? COLORS.failed : COLORS.textMuted, fontFamily: "'JetBrains Mono', monospace", fontWeight: 600 }}>{failCount}</span></div>
                      <div><span style={{ color: COLORS.textMuted, display: "block", fontSize: 10, textTransform: "uppercase", letterSpacing: 0.8, marginBottom: 4 }}>Error Rate</span><span style={{ color: errRate > 5 ? COLORS.failed : COLORS.textMuted, fontFamily: "'JetBrains Mono', monospace", fontWeight: 600 }}>{errRate}%</span></div>
                    </div>
                    <div style={{ marginTop: 12, paddingTop: 12, borderTop: `1px solid ${COLORS.borderSubtle}`, fontSize: 11, color: COLORS.textMuted }}>
                      {ss.workflows.length} workflows: {ss.workflows.join(", ")}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* ─── TRACES PAGE ─── */}
        {page === "traces" && (
          <div style={{ maxWidth: 1100 }}>
            <h1 style={{ fontSize: 20, fontWeight: 700, color: COLORS.textBright, marginBottom: 8 }}>Trace Waterfall</h1>
            <div style={{ fontSize: 12, color: COLORS.textMuted, marginBottom: 24 }}>
              Trace: <span style={{ fontFamily: "'JetBrains Mono', monospace", color: COLORS.accent }}>{MOCK_TRACE.id}</span>
              <span style={{ margin: "0 8px" }}>·</span>{MOCK_TRACE.totalDuration}ms total
              <span style={{ margin: "0 8px" }}>·</span>{new Set(MOCK_TRACE.spans.map(s => s.subsystem)).size} subsystems
              <span style={{ margin: "0 8px" }}>·</span>{MOCK_TRACE.spans.length} runs
            </div>

            <div style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 8, padding: 24, overflow: "hidden" }}>
              {/* Time axis header */}
              <div style={{ display: "grid", gridTemplateColumns: "200px 1fr", gap: 16, marginBottom: 16 }}>
                <div style={{ fontSize: 10, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 0.8 }}>Subsystem / Run</div>
                <div style={{ display: "flex", justifyContent: "space-between", fontSize: 10, color: COLORS.textMuted, fontFamily: "'JetBrains Mono', monospace" }}>
                  <span>0ms</span><span>{Math.round(MOCK_TRACE.totalDuration * 0.25)}ms</span><span>{Math.round(MOCK_TRACE.totalDuration * 0.5)}ms</span><span>{Math.round(MOCK_TRACE.totalDuration * 0.75)}ms</span><span>{MOCK_TRACE.totalDuration}ms</span>
                </div>
              </div>

              {/* Spans grouped by subsystem */}
              {(() => {
                const grouped = {};
                MOCK_TRACE.spans.forEach(s => { if (!grouped[s.subsystem]) grouped[s.subsystem] = []; grouped[s.subsystem].push(s); });
                return Object.entries(grouped).map(([ssId, spans]) => (
                  <div key={ssId} style={{ marginBottom: 16 }}>
                    {/* Subsystem header */}
                    <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
                      <span style={{ width: 10, height: 10, borderRadius: 3, background: SUBSYSTEM_COLORS[ssId] || COLORS.accent }} />
                      <span style={{ fontSize: 12, fontWeight: 700, color: COLORS.textBright }}>{SUBSYSTEMS.find(s => s.id === ssId)?.name || ssId}</span>
                    </div>
                    {/* Span bars */}
                    {spans.map((span, i) => {
                      const offsetPct = (span.offset / MOCK_TRACE.totalDuration) * 100;
                      const widthPct = Math.max((span.duration / MOCK_TRACE.totalDuration) * 100, 2);
                      const spanColor = SUBSYSTEM_COLORS[ssId] || COLORS.accent;
                      const isFailed = span.status === "failed";
                      return (
                        <div key={i} style={{ display: "grid", gridTemplateColumns: "200px 1fr 60px", gap: 16, alignItems: "center", marginBottom: 4 }}>
                          <div style={{ paddingLeft: span.parentSpanId ? 20 : 0, display: "flex", alignItems: "center", gap: 6 }}>
                            {span.parentSpanId && <span style={{ color: COLORS.border, fontSize: 10 }}>└─</span>}
                            <span style={{ fontSize: 11, fontFamily: "'JetBrains Mono', monospace", color: isFailed ? COLORS.failed : COLORS.text, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{span.workflow}</span>
                          </div>
                          <div style={{ position: "relative", height: 24, background: `${COLORS.border}20`, borderRadius: 3 }}>
                            <div
                              style={{
                                position: "absolute", left: `${offsetPct}%`, width: `${widthPct}%`, height: "100%",
                                background: isFailed ? `repeating-linear-gradient(90deg, ${COLORS.failed}, ${COLORS.failed} 4px, ${COLORS.failed}60 4px, ${COLORS.failed}60 8px)` : spanColor,
                                borderRadius: 3, cursor: "pointer", transition: "opacity 0.15s",
                                opacity: 0.85,
                              }}
                              onMouseEnter={e => { e.currentTarget.style.opacity = "1"; }}
                              onMouseLeave={e => { e.currentTarget.style.opacity = "0.85"; }}
                              title={`${span.workflow}\nOffset: ${span.offset}ms\nDuration: ${span.duration}ms\nSteps: ${span.steps.join(" → ")}\nStatus: ${span.status}`}
                            >
                              <span style={{ position: "absolute", left: 6, top: 4, fontSize: 10, color: "#fff", fontWeight: 600, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis", maxWidth: "90%" }}>
                                {span.steps.join(" → ")}
                              </span>
                            </div>
                          </div>
                          <span style={{ fontSize: 10, fontFamily: "'JetBrains Mono', monospace", color: isFailed ? COLORS.failed : COLORS.textMuted, textAlign: "right" }}>{span.duration}ms</span>
                        </div>
                      );
                    })}
                  </div>
                ));
              })()}

              {/* Legend */}
              <div style={{ display: "flex", gap: 16, marginTop: 16, paddingTop: 12, borderTop: `1px solid ${COLORS.border}`, fontSize: 10, color: COLORS.textMuted }}>
                {SUBSYSTEMS.map(ss => (
                  <span key={ss.id} style={{ display: "flex", alignItems: "center", gap: 4 }}>
                    <span style={{ width: 10, height: 10, borderRadius: 2, background: SUBSYSTEM_COLORS[ss.id] }} />{ss.name}
                  </span>
                ))}
                <span style={{ display: "flex", alignItems: "center", gap: 4 }}>
                  <span style={{ width: 10, height: 10, borderRadius: 2, background: `repeating-linear-gradient(90deg, ${COLORS.failed}, ${COLORS.failed} 3px, ${COLORS.failed}60 3px, ${COLORS.failed}60 6px)` }} />Failed
                </span>
              </div>
            </div>
          </div>
        )}

        {page === "runs" && (
          <div>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
              <h1 style={{ fontSize: 20, fontWeight: 700, color: COLORS.textBright }}>Runs</h1>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ width: 8, height: 8, borderRadius: "50%", background: COLORS.succeeded, animation: "pulse 2s infinite" }} />
                <span style={{ fontSize: 12, color: COLORS.textMuted }}>Live</span>
              </div>
            </div>

            {/* Advanced Search Bar */}
            <AdvancedSearchBar
              searchQuery={searchQuery}
              setSearchQuery={setSearchQuery}
              queryMode={queryMode}
              setQueryMode={setQueryMode}
              resultCount={searchQuery ? filteredRuns.length : null}
            />

            <div style={{ display: "flex", gap: 8, marginBottom: 16, flexWrap: "wrap" }}>
              {["all", "running", "succeeded", "failed", "paused"].map(f => (
                <button key={f} onClick={() => setFilter(f)} style={{
                  padding: "6px 14px", fontSize: 12, fontWeight: 600, borderRadius: 20,
                  background: filter === f ? COLORS.accentMuted : COLORS.surface,
                  color: filter === f ? COLORS.accent : COLORS.textMuted,
                  border: `1px solid ${filter === f ? COLORS.accent + "40" : COLORS.border}`,
                  cursor: "pointer", textTransform: "capitalize", transition: "all 0.15s",
                }}>{f}{f !== "all" && ` (${runs.filter(r => r.status === f).length})`}</button>
              ))}
            </div>

            <div style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 8, overflow: "hidden" }}>
              <div style={{
                display: "grid", gridTemplateColumns: "100px 70px 160px 160px 80px 70px",
                padding: "10px 16px", borderBottom: `1px solid ${COLORS.border}`,
                fontSize: 11, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 0.8,
              }}>
                <span>Status</span><span>ID</span><span>Workflow</span><span>Step</span><span>Started</span><span>Duration</span>
              </div>
              <div style={{ maxHeight: "calc(100vh - 220px)", overflow: "auto" }}>
                {filteredRuns.map(r => (
                  <RunRow key={r.id} run={r} selected={selectedRun?.id === r.id} onClick={setSelectedRun} />
                ))}
              </div>
            </div>
          </div>
        )}

        {page === "workflows" && (
          <div>
            <h1 style={{ fontSize: 20, fontWeight: 700, color: COLORS.textBright, marginBottom: 24 }}>Workflow Catalog</h1>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 12 }}>
              {WORKFLOWS.map(wf => {
                const wfRuns = runs.filter(r => r.workflow === wf);
                const failCount = wfRuns.filter(r => r.status === "failed").length;
                return (
                  <div key={wf} style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 8, padding: 20, cursor: "pointer", transition: "all 0.15s" }}
                    onMouseEnter={e => { e.currentTarget.style.borderColor = COLORS.accent + "60"; }}
                    onMouseLeave={e => { e.currentTarget.style.borderColor = COLORS.border; }}
                  >
                    <div style={{ fontSize: 14, fontWeight: 600, color: COLORS.textBright, marginBottom: 8 }}>{wf}</div>
                    <div style={{ fontSize: 12, color: COLORS.textMuted, marginBottom: 12 }}>{STEPS[wf].length} steps</div>
                    <div style={{ display: "flex", gap: 12, fontSize: 12 }}>
                      <span style={{ color: COLORS.succeeded }}>✓ {wfRuns.filter(r => r.status === "succeeded").length}</span>
                      <span style={{ color: COLORS.running }}>◉ {wfRuns.filter(r => r.status === "running").length}</span>
                      <span style={{ color: failCount > 0 ? COLORS.failed : COLORS.textMuted }}>✗ {failCount}</span>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {page === "analytics" && (
          <div style={{ maxWidth: 1100 }}>
            <h1 style={{ fontSize: 20, fontWeight: 700, color: COLORS.textBright, marginBottom: 24 }}>Analytics</h1>

            {/* ── STEP DURATION & LATENCY CARD ── */}
            <StepDurationCard />

            {/* ── STEP FAILURE HEATMAP ── */}
            <div style={{ background: COLORS.surface, border: `1px solid ${COLORS.border}`, borderRadius: 8, padding: 24, marginBottom: 16 }}>
              <div style={{ fontSize: 12, fontWeight: 700, color: COLORS.textMuted, textTransform: "uppercase", letterSpacing: 1, marginBottom: 16 }}>Step Failure Heatmap (Last 24h)</div>
              <div style={{ display: "grid", gridTemplateColumns: "120px repeat(12, 1fr)", gap: 3 }}>
                <div />
                {Array.from({ length: 12 }, (_, i) => (
                  <div key={i} style={{ fontSize: 10, color: COLORS.textMuted, textAlign: "center" }}>{String((i * 2) % 24).padStart(2, "0")}:00</div>
                ))}
                {["validate", "check-balance", "charge-card", "notify", "reconcile"].map(step => (
                  [
                    <div key={step} style={{ fontSize: 11, fontFamily: "'JetBrains Mono', monospace", color: COLORS.text, display: "flex", alignItems: "center" }}>{step}</div>,
                    ...Array.from({ length: 12 }, (_, i) => {
                      const heat = step === "charge-card" ? (i >= 3 && i <= 6 ? 0.6 + Math.random() * 0.4 : Math.random() * 0.1) : Math.random() * 0.15;
                      const r = Math.round(heat * 200 + 20);
                      const g = Math.round((1 - heat) * 150 + 30);
                      return (
                        <div key={`${step}-${i}`} style={{
                          height: 28, borderRadius: 3,
                          background: heat < 0.05 ? COLORS.borderSubtle : `rgba(${r}, ${g}, 40, ${0.3 + heat * 0.7})`,
                          cursor: "pointer", transition: "transform 0.1s",
                        }}
                          onMouseEnter={e => { e.currentTarget.style.transform = "scale(1.1)"; }}
                          onMouseLeave={e => { e.currentTarget.style.transform = "none"; }}
                          title={`${step} @ ${String((i * 2) % 24).padStart(2, "0")}:00 — ${Math.round(heat * 100)}% failure rate`}
                        />
                      );
                    })
                  ]
                )).flat()}
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 12, justifyContent: "flex-end" }}>
                <span style={{ fontSize: 10, color: COLORS.textMuted }}>Low</span>
                <div style={{ display: "flex", gap: 2 }}>
                  {[0.05, 0.2, 0.4, 0.6, 0.8, 1].map(h => (
                    <div key={h} style={{ width: 16, height: 10, borderRadius: 2, background: `rgba(${Math.round(h * 200 + 20)}, ${Math.round((1 - h) * 150 + 30)}, 40, ${0.3 + h * 0.7})` }} />
                  ))}
                </div>
                <span style={{ fontSize: 10, color: COLORS.textMuted }}>High</span>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* Run Detail Slide-over */}
      {selectedRun && <RunDetail run={selectedRun} onClose={() => setSelectedRun(null)} />}

      {/* Global Search Palette */}
      <SearchPalette isOpen={searchOpen} onClose={() => setSearchOpen(false)}
        onNavigate={(item) => {
          if (item.type === "advanced") {
            setPage("runs");
            setSearchQuery(item.query);
          } else if (item.type === "run") {
            setPage("runs");
            setSelectedRun(runs.find(r => r.id === item.id) || { ...item, workflow: item.workflow, foreignId: "ord-" + item.id, startedAgo: 60, duration: 3200, retries: 1, currentStep: item.step || "charge-card" });
          }
        }}
      />
    </div>
  );
}
