/**
 * Terminal dashboard — renders an in-place ASCII box that updates after each tick.
 * No external dependencies: pure ANSI escape codes + Unicode box-drawing.
 */

// ── ANSI helpers ──────────────────────────────────────────────────────────────

const A = {
  reset:   '\x1b[0m',
  bold:    '\x1b[1m',
  dim:     '\x1b[2m',
  cyan:    '\x1b[36m',
  yellow:  '\x1b[33m',
  green:   '\x1b[32m',
  red:     '\x1b[31m',
  blue:    '\x1b[34m',
  magenta: '\x1b[35m',
  white:   '\x1b[97m',
  gray:    '\x1b[90m',
};

function c(color: string, text: string): string {
  return color + text + A.reset;
}

/** Strip ANSI escape codes to measure visible length. */
function visibleLen(s: string): number {
  // eslint-disable-next-line no-control-regex
  return s.replace(/\x1b\[[0-9;]*m/g, '').length;
}

/** Pad a string to `width` visible characters. */
function pad(s: string, width: number, char = ' '): string {
  const visible = visibleLen(s);
  return visible >= width ? s : s + char.repeat(width - visible);
}

// ── Public types ──────────────────────────────────────────────────────────────

export interface DashboardState {
  tick:         number;
  agentName:    string;
  faction:      string;
  credits:      number;
  xp:           number;
  shipName:     string;
  shipClass:    string;
  hullPct:      number;
  hullCur:      number;
  hullMax:      number;
  systemID:     string;
  galaxyID:     string;
  planetID:     string | null;
  recentActions: string[];  // newest first, max 5
  lastDecision:  string;    // last LLM-chosen action
  wsConnected:   boolean;
}

// ── Dashboard class ───────────────────────────────────────────────────────────

export class Dashboard {
  private lineCount = 0;
  private started   = false;

  /** Render or re-render the dashboard in-place. */
  render(state: DashboardState): void {
    const cols   = process.stdout.columns ?? 72;
    const width  = Math.min(Math.max(cols - 2, 60), 90);
    const lines  = this.build(state, width);

    if (this.started && this.lineCount > 0) {
      // Move cursor up to overwrite previous render.
      process.stdout.write(`\x1b[${this.lineCount}A\x1b[0J`);
    }
    this.started   = true;
    this.lineCount = lines.length;
    process.stdout.write(lines.join('\n') + '\n');
  }

  /** Append a plain log line below the dashboard without disturbing it. */
  log(text: string): void {
    process.stdout.write(text + '\n');
    this.lineCount++;
  }

  private build(s: DashboardState, width: number): string[] {
    const inner = width - 2; // inside the box borders
    const half  = Math.floor(inner / 2);

    const hullBar  = this.hullBar(s.hullPct, 12);
    const wsIcon   = s.wsConnected ? c(A.green, '●') : c(A.red, '●');
    const tickStr  = `Tick ${c(A.yellow + A.bold, String(s.tick))}`;

    // ── Header ────────────────────────────────────────────────────────────────
    const title    = ` ${c(A.cyan + A.bold, '🌌 GateWanderers Agent')}  ${tickStr}  ${wsIcon} `;
    const lines: string[] = [
      c(A.blue, '╔' + '═'.repeat(width - 2) + '╗'),
      c(A.blue, '║') + pad(title, inner) + c(A.blue, '║'),
      c(A.blue, '╠' + '═'.repeat(half) + '╦' + '═'.repeat(inner - half - 1) + '╣'),
    ];

    // ── Left column: agent/ship stats ─────────────────────────────────────────
    const leftW = half;
    const rightW = inner - half - 1;

    const leftRows = [
      `Agent: ${c(A.white + A.bold, s.agentName)}`,
      `Faction: ${c(A.magenta, s.faction)}`,
      `Credits: ${c(A.yellow, s.credits.toLocaleString())}`,
      `XP: ${c(A.cyan, s.xp.toLocaleString())}`,
      `Ship: ${c(A.white, s.shipName)}`,
      `Hull: ${hullBar} ${c(s.hullPct < 30 ? A.red : s.hullPct < 60 ? A.yellow : A.green, s.hullPct + '%')}`,
      `System: ${c(A.cyan, s.systemID)}`,
      s.planetID ? `Planet: ${c(A.green, s.planetID)}` : `Galaxy: ${c(A.gray, s.galaxyID)}`,
    ];

    // ── Right column: recent actions ──────────────────────────────────────────
    const rightLabel = c(A.white + A.bold, 'Recent Actions:');
    const actionRows = s.recentActions.slice(0, 5).map((a) => {
      const icon = a.startsWith('✓') ? c(A.green, '✓') : a.startsWith('✗') ? c(A.red, '✗') : c(A.gray, '•');
      const text = a.replace(/^[✓✗•]\s*/, '');
      return `${icon} ${c(A.gray, text)}`;
    });
    if (actionRows.length === 0) actionRows.push(c(A.gray, '(no actions yet)'));

    const rightRows = [rightLabel, ...actionRows];

    // Fill to equal height
    const rowCount = Math.max(leftRows.length, rightRows.length);
    while (leftRows.length  < rowCount) leftRows.push('');
    while (rightRows.length < rowCount) rightRows.push('');

    for (let i = 0; i < rowCount; i++) {
      const left  = ' ' + pad(leftRows[i],  leftW  - 1);
      const right = ' ' + pad(rightRows[i], rightW - 1);
      lines.push(c(A.blue, '║') + left + c(A.blue, '║') + right + c(A.blue, '║'));
    }

    // ── Divider + decision ────────────────────────────────────────────────────
    lines.push(c(A.blue, '╠' + '═'.repeat(half) + '╩' + '═'.repeat(inner - half - 1) + '╣'));

    const decisionLabel = c(A.dim, 'Last Decision: ');
    const decisionText  = s.lastDecision
      ? c(A.white, s.lastDecision.slice(0, inner - 16))
      : c(A.gray, '—');
    lines.push(c(A.blue, '║') + pad(' ' + decisionLabel + decisionText, inner) + c(A.blue, '║'));

    lines.push(c(A.blue, '╚' + '═'.repeat(width - 2) + '╝'));

    return lines;
  }

  private hullBar(pct: number, len: number): string {
    const filled = Math.round((pct / 100) * len);
    const empty  = len - filled;
    const color  = pct < 30 ? A.red : pct < 60 ? A.yellow : A.green;
    return c(A.gray, '[') + c(color, '█'.repeat(filled)) + c(A.gray, '░'.repeat(empty) + ']');
  }
}
