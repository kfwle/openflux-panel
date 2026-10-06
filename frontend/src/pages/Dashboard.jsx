import { motion } from 'framer-motion';
import { KeyRound, Wifi, ArrowDownToLine, Activity } from 'lucide-react';
import { fmtBytes } from '../api';
import { AreaChart, Empty, StatusPill, TrChips, Usage, Btn } from '../components';

const fade = { initial: { opacity: 0, y: 14 }, animate: { opacity: 1, y: 0 } };

function Stat({ icon: Icon, label, value, sub, delay }) {
  return (
    <motion.div {...fade} transition={{ duration: 0.4, delay }} className="card p-5">
      <div className="flex items-center gap-2 text-muted">
        <Icon size={15} />
        <span className="text-xs font-medium uppercase tracking-wide">{label}</span>
      </div>
      <div className="text-[26px] font-semibold tracking-tight mt-2">{value}</div>
      <div className="text-[13px] text-muted mt-0.5">{sub}</div>
    </motion.div>
  );
}

export default function Dashboard({ stats, keys, history, goKeys, onNew }) {
  const act = {};
  keys.forEach((k) => {
    if (k.connected && k.active_transport) act[k.active_transport] = (act[k.active_transport] || 0) + 1;
  });
  const via = Object.keys(act).length ? Object.entries(act).map(([t, n]) => `${t} × ${n}`).join(', ') : '—';
  const binMissing = keys.some((k) => (k.error || '').includes('binary not found'));

  return (
    <div>
      {binMissing && (
        <motion.div {...fade} className="card p-4 mb-5 text-sm border-l-4" style={{ borderLeftColor: 'var(--color-ochre)' }}>
          <b>Не найден бинарь openflux.</b> Ссылки и разбор работают, но exit-процессы не запускаются.
          Укажите путь в Настройках.
        </motion.div>
      )}
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-4 mb-5">
        <Stat icon={KeyRound} label="Ключи" value={stats.keys_total ?? '–'} sub={<>активно {stats.keys_active ?? '–'} · online {stats.online ?? '–'}</>} delay={0} />
        <Stat icon={ArrowDownToLine} label="Трафик всего" value={fmtBytes((stats.total_up || 0) + (stats.total_down || 0))} sub={<>↓ {fmtBytes(stats.total_down)} · ↑ {fmtBytes(stats.total_up)}</>} delay={0.05} />
        <Stat icon={Activity} label="Сегодня" value={fmtBytes((stats.today_up || 0) + (stats.today_down || 0))} sub={<>↓ {fmtBytes(stats.today_down)} · ↑ {fmtBytes(stats.today_up)}</>} delay={0.1} />
        <Stat icon={Wifi} label="Маршрут" value={<span className="mono text-lg">{via}</span>} sub={stats.bin ? `bin: ${stats.bin}` : 'ядро'} delay={0.15} />
      </div>

      <motion.div {...fade} transition={{ duration: 0.4, delay: 0.15 }} className="card p-5 mb-5">
        <div className="font-medium mb-3">Трафик за 7 дней</div>
        <AreaChart history={history} />
      </motion.div>

      <motion.div {...fade} transition={{ duration: 0.4, delay: 0.2 }} className="card overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b border-line">
          <div className="font-medium">Ключи</div>
          <div className="flex gap-2">
            <Btn onClick={goKeys}>Все ключи</Btn>
            <Btn kind="primary" onClick={onNew}>Новый ключ</Btn>
          </div>
        </div>
        {keys.length === 0 ? (
          <Empty title="Пока нет ключей" hint="Каждый ключ — это отдельный exit-процесс ядра. Создайте первый, чтобы получить ссылку для приложения." action={<Btn kind="primary" onClick={onNew}>Создать ключ</Btn>} />
        ) : (
          <div className="overflow-x-auto">
            <table className="ledger">
              <thead><tr><th>Ключ</th><th>Статус</th><th>Транспорты</th><th>Трафик</th></tr></thead>
              <tbody>
                {keys.slice(0, 8).map((k) => (
                  <tr key={k.id} className="hover:bg-paper/60 transition cursor-pointer" onClick={goKeys}>
                    <td>
                      <div className="font-medium">{k.name}</div>
                      <div className="mono text-xs text-faint">{k.mode}/{k.codec}{k.direct_port ? ` · :${k.direct_port}` : ''}{k.connected && k.active_transport ? ` · via ${k.active_transport}` : ''}</div>
                      {k.error && <div className="text-xs text-brick mt-0.5">{k.error}</div>}
                    </td>
                    <td><StatusPill status={k.status} connected={k.connected} /></td>
                    <td><TrChips k={k} /></td>
                    <td><Usage k={k} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </motion.div>
    </div>
  );
}
