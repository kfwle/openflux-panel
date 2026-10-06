import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import QRCode from 'react-qr-code';
import { Plus, Search, QrCode, ScrollText, RotateCcw, Pencil, Trash2, Link2, Dices } from 'lucide-react';
import { api, GB, randomSecret, TRANSPORTS, TR_HINT } from '../api';
import { Modal, ModalHead, Btn, StatusPill, TrChips, Usage, Empty, TrIcon } from '../components';

/* ================= list ================= */
export default function Keys({ keys, reload, notify, openSignal }) {
  const [q, setQ] = useState('');
  const [form, setForm] = useState(null); // null | {key?}
  const [qr, setQr] = useState(null);
  const [parseOpen, setParseOpen] = useState(false);
  const [logs, setLogs] = useState(null);

  useEffect(() => {
    if (openSignal > 0) setForm({});
  }, [openSignal]);

  const list = keys.filter((k) => (k.name + k.id).toLowerCase().includes(q.toLowerCase()));

  const act = async (id, action, confirmMsg) => {
    if (confirmMsg && !confirm(confirmMsg)) return;
    try {
      const m = action === 'delete' ? 'DELETE' : 'POST';
      const url = action === 'delete' ? `/api/keys/${id}` : `/api/keys/${id}/${action}`;
      await api(m, url);
      setTimeout(reload, 700);
    } catch (e) { notify(e.message); }
  };

  const openQr = async (k) => {
    try {
      const j = await api('GET', `/api/keys/${k.id}/link`);
      setQr({ name: k.name, ...j });
    } catch (e) { notify(e.message); }
  };

  const openLogs = async (k) => {
    setLogs({ name: k.name, text: 'Загрузка…' });
    try {
      const j = await api('GET', `/api/keys/${k.id}/logs`);
      setLogs({ name: k.name, text: j.logs || '(пока пусто — процесс ещё ничего не писал)' });
    } catch (e) { setLogs({ name: k.name, text: 'Ошибка: ' + e.message }); }
  };

  return (
    <div>
      <div className="flex items-center gap-3 mb-5 flex-wrap">
        <div className="relative flex-1 min-w-[200px] max-w-xs">
          <Search size={15} className="absolute left-3.5 top-1/2 -translate-y-1/2 text-faint" />
          <input className="inp !pl-9" placeholder="Поиск…" value={q} onChange={(e) => setQ(e.target.value)} />
        </div>
        <div className="flex gap-2 ml-auto">
          <Btn onClick={() => setParseOpen(true)}><Link2 size={15} /> Разобрать ссылку</Btn>
          <Btn kind="primary" onClick={() => setForm({})}><Plus size={15} /> Новый ключ</Btn>
        </div>
      </div>

      <div className="card overflow-hidden">
        {list.length === 0 ? (
          <Empty title="Ничего не найдено" hint="Создайте первый ключ — каждый ключ это отдельный exit-процесс ядра со своим секретом и портом." action={<Btn kind="primary" onClick={() => setForm({})}><Plus size={15} /> Создать ключ</Btn>} />
        ) : (
          <div className="overflow-x-auto">
            <table className="ledger">
              <thead><tr><th>Ключ</th><th>Статус</th><th>Транспорты</th><th>Трафик</th><th>IP</th><th>До</th><th className="!text-right">Действия</th></tr></thead>
              <tbody>
                <AnimatePresence initial={false}>
                  {list.map((k) => (
                    <motion.tr key={k.id} layout initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="hover:bg-paper/60 transition">
                      <td>
                        <button className="font-medium hover:text-pine transition text-left" onClick={() => openQr(k)}>{k.name}</button>
                        <div className="mono text-xs text-faint">{k.mode}/{k.codec}{k.direct_port ? ` · :${k.direct_port}` : ''}{k.connected && k.active_transport ? ` · via ${k.active_transport}` : ''}</div>
                        {k.error && <div className="text-xs text-brick mt-0.5">{k.error}</div>}
                      </td>
                      <td><StatusPill status={k.status} connected={k.connected} /></td>
                      <td><div className="max-w-[260px]"><TrChips k={k} /></div></td>
                      <td><Usage k={k} /></td>
                      <td>
                        <div className={`text-sm font-medium ${(k.ip_limit > 0 && (k.online_ips || []).length > k.ip_limit) ? 'text-brick' : ''}`}>
                          {(k.online_ips || []).length}{k.ip_limit > 0 ? ` / ${k.ip_limit}` : <span className="text-faint font-normal"> / ∞</span>}
                        </div>
                        {(k.online_ips || []).length > 0 && <div className="mono text-[11px] text-faint">{k.online_ips.slice(0, 3).join(', ')}{k.online_ips.length > 3 ? '…' : ''}</div>}
                      </td>
                      <td className="text-[13px] whitespace-nowrap text-muted">{k.expiry ? k.expiry.slice(0, 10) : '∞'}</td>
                      <td>
                        <div className="flex gap-1 justify-end">
                          <IconBtn title="QR и ссылка" onClick={() => openQr(k)}><QrCode size={15} /></IconBtn>
                          <IconBtn title="Лог процесса" onClick={() => openLogs(k)}><ScrollText size={15} /></IconBtn>
                          <IconBtn title="Перезапустить" onClick={() => act(k.id, 'restart')}><RotateCcw size={15} /></IconBtn>
                          <IconBtn title="Изменить" onClick={() => setForm({ key: k })}><Pencil size={15} /></IconBtn>
                          <IconBtn title="Удалить" danger onClick={() => act(k.id, 'delete', `Удалить ключ «${k.name}»? Процесс будет остановлен.`)}><Trash2 size={15} /></IconBtn>
                        </div>
                      </td>
                    </motion.tr>
                  ))}
                </AnimatePresence>
              </tbody>
            </table>
          </div>
        )}
      </div>

      <KeyFormModal form={form} onClose={() => setForm(null)} reload={reload} notify={notify} />
      <QrModal qr={qr} onClose={() => setQr(null)} notify={notify} />
      <ParseModal open={parseOpen} onClose={() => setParseOpen(false)} notify={notify} />
      <Modal open={!!logs} onClose={() => setLogs(null)}>
        <ModalHead title={`Лог · ${logs?.name}`} onClose={() => setLogs(null)} />
        <pre className="mono text-xs p-4 rounded-xl overflow-auto scroll-thin bg-paper border border-line" style={{ maxHeight: '60vh' }}>{logs?.text}</pre>
      </Modal>
    </div>
  );
}

function IconBtn({ children, title, danger, onClick }) {
  return (
    <motion.button
      whileTap={{ scale: 0.92 }} title={title} onClick={onClick}
      className={`w-8 h-8 rounded-lg border flex items-center justify-center transition ${danger ? 'border-brick/25 text-brick hover:bg-brick hover:text-white' : 'border-line text-muted hover:text-pine hover:border-pine/40 hover:bg-pine-soft/50'}`}
    >{children}</motion.button>
  );
}

/* ================= key form ================= */
function KeyFormModal({ form, onClose, reload, notify }) {
  const k = form?.key;
  const [name, setName] = useState('');
  const [mode, setMode] = useState('l3');
  const [codec, setCodec] = useState('batched');
  const [secret, setSecret] = useState('');
  const [ctx, setCtx] = useState('');
  const [port, setPort] = useState('0');
  const [limit, setLimit] = useState('0');
  const [iplimit, setIplimit] = useState('0');
  const [exp, setExp] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [trs, setTrs] = useState([{ type: 'direct', url: '', priority: 100 }]);
  const [err, setErr] = useState('');
  const [inited, setInited] = useState(false);

  if (form && !inited) {
    setInited(true);
    if (k) {
      setName(k.name); setMode(k.mode); setCodec(k.codec || 'batched');
      setSecret(k.secret || ''); setCtx(k.context || ''); setPort(String(k.direct_port || 0));
      setLimit(k.traffic_limit ? String(k.traffic_limit / GB) : '0');
      setIplimit(String(k.ip_limit || 0)); setExp(k.expiry ? k.expiry.slice(0, 10) : '');
      setEnabled(!!k.enabled);
      setTrs((k.transports || []).map((t) => ({ type: t.type, url: t.url || '', priority: t.priority || 50 })));
    } else {
      setName(''); setSecret(randomSecret()); setMode('l3'); setCodec('batched');
      setCtx(''); setPort('0'); setLimit('0'); setIplimit('0'); setExp(''); setEnabled(true);
      setTrs([{ type: 'direct', url: '', priority: 100 }]);
    }
    setErr('');
  }
  if (!form && inited) setInited(false);

  const save = async () => {
    const body = {
      name: name.trim(), secret: secret.trim(), context: ctx.trim(), codec, mode,
      transports: trs.map((t) => ({ type: t.type, url: t.url.trim(), priority: parseInt(t.priority) || 50 })),
      direct_port: parseInt(port) || 0,
      traffic_limit: Math.round((parseFloat(limit) || 0) * GB),
      ip_limit: parseInt(iplimit) || 0,
      expiry: exp ? new Date(exp + 'T23:59:59').toISOString() : '',
      enabled,
    };
    try {
      if (k) await api('PUT', `/api/keys/${k.id}`, body);
      else await api('POST', '/api/keys', body);
      onClose(); reload();
    } catch (e) { setErr(e.message); }
  };

  return (
    <Modal open={!!form} onClose={onClose} wide>
      <ModalHead title={k ? `Изменить · ${k.name}` : 'Новый ключ'} onClose={onClose} />
      <div className="grid md:grid-cols-2 gap-4">
        <div><label className="lbl">Имя</label><input className="inp" value={name} onChange={(e) => setName(e.target.value)} placeholder="client-1" /></div>
        <div><label className="lbl">Режим exit</label>
          <select className="inp" value={mode} onChange={(e) => setMode(e.target.value)}>
            <option value="l3">l3 — raw, Linux + root</option>
            <option value="l4">l4 — proxy, везде</option>
            <option value="stream">stream — PHP-нода</option>
          </select>
        </div>
        <div><label className="lbl">Секрет, мин. 16 символов</label>
          <div className="flex gap-2"><input className="inp mono" value={secret} onChange={(e) => setSecret(e.target.value)} /><Btn title="Сгенерировать случайно" onClick={() => setSecret(randomSecret())}><Dices size={15} /></Btn></div>
        </div>
        <div><label className="lbl">Кодек</label>
          <select className="inp" value={codec} onChange={(e) => setCodec(e.target.value)}>
            <option value="batched">batched + zstd</option>
            <option value="legacy">legacy LZ4</option>
          </select>
        </div>
        <div><label className="lbl">Контекст (необязательно)</label><input className="inp mono" value={ctx} onChange={(e) => setCtx(e.target.value)} placeholder="авто: URL топ-приоритета" /></div>
        <div><label className="lbl">Direct-порт (0 — авто)</label><input type="number" className="inp mono" value={port} onChange={(e) => setPort(e.target.value)} /></div>
        <div><label className="lbl">Лимит трафика, ГБ (0 — ∞)</label><input type="number" step="0.1" className="inp mono" value={limit} onChange={(e) => setLimit(e.target.value)} /></div>
        <div><label className="lbl">Лимит IP, шт (0 — ∞)</label><input type="number" className="inp mono" value={iplimit} onChange={(e) => setIplimit(e.target.value)} /></div>
        <div><label className="lbl">Срок действия</label><input type="date" className="inp" value={exp} onChange={(e) => setExp(e.target.value)} /></div>
        <div className="flex items-end pb-2"><label className="text-sm flex gap-2 items-center cursor-pointer"><input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} className="w-4 h-4 accent-[#2f665c]" /> Включён</label></div>
      </div>
      <div className="mt-5">
        <div className="flex items-center justify-between mb-2">
          <label className="lbl !mb-0">Транспорты · приоритет выше — главнее</label>
          <Btn onClick={() => setTrs([...trs, { type: 'yandex', url: '', priority: 50 }])}><Plus size={14} /> Добавить</Btn>
        </div>
        <div className="space-y-2">
          {trs.map((t, i) => (
            <motion.div key={i} layout className="flex gap-2 items-start bg-paper border border-line rounded-xl p-2.5">
              <div className="pt-2"><TrIcon type={t.type} size={20} /></div>
              <select className="inp !w-32" value={t.type} onChange={(e) => { const n = [...trs]; n[i].type = e.target.value; setTrs(n); }}>
                {TRANSPORTS.map((x) => <option key={x}>{x}</option>)}
              </select>
              <div className="flex-1">
                <input className="inp mono" placeholder="URL / token|uid" value={t.url} onChange={(e) => { const n = [...trs]; n[i].url = e.target.value; setTrs(n); }} />
                <div className="text-[11px] text-faint mt-1">{TR_HINT[t.type]}</div>
              </div>
              <input type="number" className="inp mono !w-[74px]" title="Приоритет failover: чем больше число, тем главнее транспорт. Обычно главному — 100, запасным — 50" value={t.priority} onChange={(e) => { const n = [...trs]; n[i].priority = e.target.value; setTrs(n); }} />
              <Btn onClick={() => setTrs(trs.filter((_, j) => j !== i))}>✕</Btn>
            </motion.div>
          ))}
        </div>
      </div>
      {err && <div className="text-sm text-brick mt-3">{err}</div>}
      <div className="flex gap-2 mt-5 justify-end"><Btn onClick={onClose}>Отмена</Btn><Btn kind="primary" onClick={save}>Сохранить</Btn></div>
    </Modal>
  );
}

/* ================= QR modal ================= */
async function copyText(t) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(t);
      return true;
    }
    throw new Error('no clipboard');
  } catch {
    // fallback для http без TLS: скрытый textarea + execCommand
    const ta = document.createElement('textarea');
    ta.value = t;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    let ok = false;
    try { ok = document.execCommand('copy'); } catch { ok = false; }
    ta.remove();
    return ok;
  }
}

function QrModal({ qr, onClose, notify }) {
  if (!qr) return <Modal open={false} onClose={onClose} />;
  const copy = async (okMsg, v) => notify(await copyText(v) ? okMsg : 'Не скопировалось — выделите текст вручную');
  return (
    <Modal open={!!qr} onClose={onClose}>
      <ModalHead title={qr.name} onClose={onClose} />
      <div className="text-center">
        <motion.div initial={{ scale: 0.94, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} className="inline-block p-4 bg-white border border-line rounded-2xl">
          <QRCode value={qr.link} size={210} />
        </motion.div>
        <div className="mono text-[11px] break-all p-3 rounded-xl mt-4 text-left max-h-24 overflow-y-auto scroll-thin bg-paper border border-line">{qr.link}</div>
        <div className="flex gap-2 mt-3 justify-center flex-wrap">
          <Btn kind="primary" onClick={() => copy('Ссылка скопирована', qr.link)}>Копировать ссылку</Btn>
          <Btn onClick={() => copy('JSON скопирован', JSON.stringify(qr.config, null, 2))}>JSON конфига</Btn>
        </div>
        <div className="text-xs text-muted mt-3">Отсканируйте в OpenFluxAndroid. Ссылка — это секрет: храните как пароль.</div>
      </div>
    </Modal>
  );
}

/* ================= parse modal ================= */
function ParseModal({ open, onClose, notify }) {
  const [text, setText] = useState('');
  const [out, setOut] = useState(null);
  const [err, setErr] = useState('');
  const go = async () => {
    try {
      const j = await api('POST', '/api/parse-link', { link: text.trim() });
      setOut(j); setErr('');
    } catch (e) { setErr(e.message); setOut(null); }
  };
  return (
    <Modal open={open} onClose={onClose}>
      <ModalHead title="Разобрать ссылку" onClose={onClose} />
      <label className="lbl">Вставьте openflux://…</label>
      <textarea className="inp mono" rows={3} value={text} onChange={(e) => setText(e.target.value)} placeholder="openflux://v1/..." />
      <div className="mt-3"><Btn kind="primary" onClick={go}>Разобрать</Btn></div>
      {err && <div className="text-sm text-brick mt-2">{err}</div>}
      {out && <pre className="mono text-xs mt-3 p-4 rounded-xl overflow-x-auto bg-paper border border-line">{JSON.stringify(out, null, 2)}</pre>}
    </Modal>
  );
}
