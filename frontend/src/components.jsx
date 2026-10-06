import { motion, AnimatePresence } from 'framer-motion';
import { X } from 'lucide-react';
import { fmtBytes } from './api';
import maxLogo from './assets/max-logo.png';
import cupsIcon from './assets/cups-favicon.ico';
import boardsIcon from './assets/boards-icon.png';

/* ---------- modal shell ---------- */
export function Modal({ open, onClose, children, wide }) {
  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          style={{ background: 'rgba(41,37,36,.38)', backdropFilter: 'blur(3px)' }}
          initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
          onClick={onClose}
        >
          <motion.div
            className={`card w-full ${wide ? 'max-w-2xl' : 'max-w-lg'} max-h-[92vh] overflow-y-auto scroll-thin p-6`}
            style={{ boxShadow: '0 24px 64px rgba(41,37,36,.18)' }}
            initial={{ opacity: 0, y: 24, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 12, scale: 0.98 }}
            transition={{ type: 'spring', stiffness: 380, damping: 32 }}
            onClick={(e) => e.stopPropagation()}
          >
            {children}
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

export function ModalHead({ title, onClose }) {
  return (
    <div className="flex items-center justify-between mb-5">
      <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
      <button onClick={onClose} className="w-8 h-8 rounded-full flex items-center justify-center text-muted hover:bg-paper transition" aria-label="Закрыть">
        <X size={17} />
      </button>
    </div>
  );
}

/* ---------- buttons ---------- */
export function Btn({ kind = 'ghost', className = '', ...p }) {
  const base = 'inline-flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition cursor-pointer border ';
  const kinds = {
    primary: 'bg-pine text-white border-pine hover:brightness-110',
    ghost: 'bg-card text-ink border-line hover:border-pine/50 hover:bg-pine-soft/40',
    danger: 'bg-brick-soft text-brick border-brick/25 hover:bg-brick hover:text-white',
  };
  return <motion.button whileTap={{ scale: 0.97 }} className={base + (kinds[kind] || kinds.ghost) + ' ' + className} {...p} />;
}

/* ---------- status pill ---------- */
const STATUS = {
  active: ['Активен', 'bg-pine-soft text-pine'],
  disabled: ['Выключен', 'bg-paper text-muted border border-line'],
  limited: ['Лимит', 'bg-ochre-soft text-ochre'],
  expired: ['Истёк', 'bg-brick-soft text-brick'],
  error: ['Ошибка', 'bg-brick-soft text-brick'],
};
export function StatusPill({ status, connected }) {
  const [label, cls] = STATUS[status] || STATUS.disabled;
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className={`text-xs font-medium px-2.5 py-1 rounded-full ${cls}`}>{label}</span>
      {connected && <span className="text-xs font-medium px-2.5 py-1 rounded-full bg-pine-soft text-pine">● online</span>}
    </span>
  );
}

/* ---------- transport chips ---------- */
export function TrChips({ k }) {
  return (
    <div className="flex gap-1.5 flex-wrap">
      {(k.transports || []).map((t, i) => {
        let lbl = t.type;
        if (t.type === 'direct') lbl = `direct · ${k.direct_port || '?'}`;
        else if (t.url) {
          try { lbl = `${t.type} · ${new URL(t.url).hostname.replace(/^www\./, '')}`; }
          catch { lbl = t.type; }
        }
        return (
          <span key={i} className="inline-flex items-center gap-1.5 mono text-[11px] px-2 py-1 rounded-md bg-paper border border-line text-muted" title={`${t.type} · приоритет ${t.priority || 50}: выше число — главнее`}>
            <TrIcon type={t.type} size={14} />{lbl} <span className="text-faint">{t.priority || ''}</span>
          </span>
        );
      })}
    </div>
  );
}

/* ---------- traffic usage ---------- */
export function Usage({ k }) {
  const used = (k.traffic_up || 0) + (k.traffic_down || 0);
  const lim = k.traffic_limit || 0;
  const pct = lim > 0 ? Math.min(100, (used / lim) * 100) : 0;
  return (
    <div className="min-w-[150px]">
      <div className="text-sm font-medium">
        {fmtBytes(used)} <span className="text-faint font-normal">/ {lim > 0 ? fmtBytes(lim) : '∞'}</span>
      </div>
      <div className="text-xs text-muted mt-0.5">↓ {fmtBytes(k.traffic_down)} · ↑ {fmtBytes(k.traffic_up)}</div>
      {lim > 0 && (
        <div className="h-1.5 rounded-full bg-paper border border-line mt-1.5 overflow-hidden">
          <motion.div
            className="h-full rounded-full"
            style={{ background: pct > 90 ? 'var(--color-brick)' : 'var(--color-pine)' }}
            initial={false} animate={{ width: `${pct}%` }} transition={{ type: 'spring', stiffness: 120, damping: 20 }}
          />
        </div>
      )}
    </div>
  );
}

/* ---------- calm SVG area chart ---------- */
export function AreaChart({ history }) {
  const W = 720, H = 200, P = 8;
  const byTs = {};
  (history || []).forEach((s) => {
    const o = byTs[s.ts] || (byTs[s.ts] = { up: 0, down: 0 });
    o.up += s.up; o.down += s.down;
  });
  const ts = Object.keys(byTs).map(Number).sort((a, b) => a - b).slice(-288);
  if (!ts.length)
    return <div className="h-[200px] flex items-center justify-center text-sm text-faint">Пока нет данных — появятся после первых замеров трафика</div>;
  const max = Math.max(1, ...ts.map((t) => byTs[t].down + byTs[t].up));
  const X = (i) => P + (i / Math.max(1, ts.length - 1)) * (W - P * 2);
  const Y = (v) => H - P - (v / max) * (H - P * 2);
  const line = (pick) => ts.map((t, i) => `${i ? 'L' : 'M'}${X(i).toFixed(1)},${Y(byTs[t][pick]).toFixed(1)}`).join(' ');
  const area = (pick) => `${line(pick)} L${X(ts.length - 1).toFixed(1)},${H - P} L${P},${H - P} Z`;
  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-[200px]">
        {[0.25, 0.5, 0.75].map((f) => (
          <line key={f} x1={P} x2={W - P} y1={H * f} y2={H * f} stroke="#ece8dd" strokeWidth="1" />
        ))}
        <motion.path d={area('down')} fill="rgba(47,102,92,.10)" initial={false} animate={{ d: area('down') }} transition={{ duration: 0.6 }} />
        <motion.path d={line('down')} fill="none" stroke="#2f665c" strokeWidth="2" strokeLinecap="round" initial={false} animate={{ d: line('down') }} transition={{ duration: 0.6 }} />
        <motion.path d={line('up')} fill="none" stroke="#b3a179" strokeWidth="1.5" strokeDasharray="5 4" strokeLinecap="round" initial={false} animate={{ d: line('up') }} transition={{ duration: 0.6 }} />
      </svg>
      <div className="flex gap-4 text-xs text-muted mt-1">
        <span className="flex items-center gap-1.5"><i className="w-4 h-0.5 rounded inline-block bg-pine" /> входящий</span>
        <span className="flex items-center gap-1.5"><i className="w-4 border-t-2 border-dashed inline-block" style={{ borderColor: '#b3a179' }} /> исходящий</span>
      </div>
    </div>
  );
}

/* ---------- transport brand icons ---------- */
export function TrIcon({ type, size = 15 }) {
  const box = { width: size, height: size, viewBox: '0 0 24 24', style: { flexShrink: 0 } };
  switch (type) {
    case 'yandex': // Яндекс — красная «Я»
      return (<svg {...box}><rect width="24" height="24" rx="6" fill="#FC3F1D" /><text x="12" y="17.5" textAnchor="middle" fontSize="14" fontWeight="800" fill="#fff" fontFamily="Arial,sans-serif">Я</text></svg>);
    case 'vyandex': // Волга — та же «Я», тёмно-синяя
      return (<svg {...box}><rect width="24" height="24" rx="6" fill="#1F3A5F" /><text x="12" y="17.5" textAnchor="middle" fontSize="14" fontWeight="800" fill="#fff" fontFamily="Arial,sans-serif">Я</text></svg>);
    case 'boards': // Доски — официальная иконка
      return (<img src={boardsIcon} alt="Yandex Boards" width={size} height={size} style={{ width: size, height: size, borderRadius: size / 4, flexShrink: 0 }} />);
    case 'mailru': // Mail.ru — синий конверт
      return (<svg {...box}><rect width="24" height="24" rx="6" fill="#005FF9" /><rect x="5" y="7.5" width="14" height="9.5" rx="2" fill="none" stroke="#fff" strokeWidth="1.8" /><path d="M6 9l6 4.5L18 9" fill="none" stroke="#fff" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'cupsonline': // Cups — официальная иконка
      return (<img src={cupsIcon} alt="cups.online" width={size} height={size} style={{ width: size, height: size, borderRadius: size / 4, flexShrink: 0 }} />);
    case 'oneme': // MAX — официальный логотип
      return (<img src={maxLogo} alt="MAX" width={size} height={size} style={{ width: size, height: size, borderRadius: size / 4, flexShrink: 0 }} />);
    case 'direct': // Direct — глобус
      return (<svg {...box}><rect width="24" height="24" rx="6" fill="#E7EFEC" /><g fill="none" stroke="#2F665C" strokeWidth="1.6"><circle cx="12" cy="12" r="6.5" /><ellipse cx="12" cy="12" rx="3" ry="6.5" /><path d="M5.5 12h13M7 8.2c3.5 1.2 6.5 1.2 10 0M7 15.8c3.5-1.2 6.5-1.2 10 0" /></g></svg>);
    default:
      return (<svg {...box}><rect width="24" height="24" rx="6" fill="#E7E1D6" /><circle cx="12" cy="12" r="3" fill="#78716C" /></svg>);
  }
}

/* ---------- empty state ---------- */
export function Empty({ title, hint, action }) {
  return (
    <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="text-center py-12">
      <div className="text-[15px] font-medium">{title}</div>
      <div className="text-sm text-muted mt-1 max-w-md mx-auto">{hint}</div>
      {action && <div className="mt-4">{action}</div>}
    </motion.div>
  );
}
