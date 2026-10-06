import { useEffect, useRef, useState } from 'react';
import { motion } from 'framer-motion';
import { api } from '../api';
import { Btn } from '../components';

export default function Settings({ settings, reload, notify }) {
  const [s, setS] = useState(null);
  const [oldPw, setOldPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [autoIp, setAutoIp] = useState(false);
  const autoTried = useRef(false);
  const cur = s || settings || {};
  const set = (k, v) => { setAutoIp(false); setS({ ...cur, [k]: v }); };

  // Если share-host пуст — определяем IP сами и сразу сохраняем
  useEffect(() => {
    if (!settings || settings.share_host || autoTried.current) return;
    autoTried.current = true;
    api('GET', '/api/public-ip')
      .then(async (j) => {
        if (!j.ip) return;
        const next = { ...settings, share_host: j.ip };
        setS(next);
        setAutoIp(true);
        try { await api('PUT', '/api/settings', next); } catch { /* тихо */ }
      })
      .catch(() => {});
  }, [settings]);

  const save = async () => {
    try {
      await api('PUT', '/api/settings', {
        share_host: (cur.share_host || '').trim(),
        direct_from: parseInt(cur.direct_from) || 20000,
        direct_to: parseInt(cur.direct_to) || 21000,
        openflux_bin: (cur.openflux_bin || '').trim() || './openflux',
        default_mode: cur.default_mode || 'l3',
        default_codec: cur.default_codec || 'batched',
        local_ip: (cur.local_ip || '').trim(),
        auto_disable: !!cur.auto_disable,
        auto_delete_days: parseInt(cur.auto_delete_days) || 0,
        poll_interval_sec: parseInt(cur.poll_interval_sec) || 10,
      });
      notify('Настройки сохранены'); reload();
    } catch (e) { notify(e.message); }
  };

  const detect = async () => {
    try {
      const j = await api('GET', '/api/public-ip');
      if (j.ip) set('share_host', j.ip);
      else notify('Не удалось определить IP');
    } catch (e) { notify(e.message); }
  };

  const changePw = async () => {
    try {
      await api('POST', '/api/change-password', { old: oldPw, new: newPw });
      setOldPw(''); setNewPw(''); notify('Пароль изменён');
    } catch (e) { notify(e.message); }
  };

  return (
    <div className="grid md:grid-cols-2 gap-4 items-start">
      <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} className="card p-6 space-y-4">
        <div className="font-medium">Exit-нода</div>
        <div>
          <label className="lbl">Share-host — белый IP для direct</label>
          <div className="flex gap-2">
            <input className="inp mono" value={cur.share_host || ''} onChange={(e) => set('share_host', e.target.value)} placeholder="определяется…" />
            <Btn onClick={detect}>Определить</Btn>
          </div>
          {autoIp && <div className="text-xs text-pine mt-1.5">IP определён автоматически и сохранён</div>}
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div><label className="lbl">Порты direct от</label><input type="number" className="inp mono" value={cur.direct_from || ''} onChange={(e) => set('direct_from', e.target.value)} /></div>
          <div><label className="lbl">Порты direct до</label><input type="number" className="inp mono" value={cur.direct_to || ''} onChange={(e) => set('direct_to', e.target.value)} /></div>
        </div>
        <div><label className="lbl">Путь к бинарю openflux</label><input className="inp mono" value={cur.openflux_bin || ''} onChange={(e) => set('openflux_bin', e.target.value)} placeholder="./openflux" /></div>
        <div>
          <label className="lbl">Local IP для l3 (необязательно)</label>
          <input className="inp mono" value={cur.local_ip || ''} onChange={(e) => set('local_ip', e.target.value)} placeholder="напр. 10.0.0.5" />
          <div className="text-xs text-faint mt-1.5">Выделенный egress-IP + правило <span className="mono">iptables … -s IP -j DROP</span> против kernel RST. Без него — правило на весь хост (см. помощь).</div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div><label className="lbl">Режим по умолчанию</label>
            <select className="inp" value={cur.default_mode || 'l3'} onChange={(e) => set('default_mode', e.target.value)}>
              <option value="l3">l3 — raw, Linux + root</option>
              <option value="l4">l4 — proxy, везде</option>
            </select>
          </div>
          <div><label className="lbl">Кодек</label>
            <select className="inp" value={cur.default_codec || 'batched'} onChange={(e) => set('default_codec', e.target.value)}>
              <option value="batched">batched + zstd</option>
              <option value="legacy">legacy LZ4</option>
            </select>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3 items-end">
          <div><label className="lbl">Опрос IPC, сек</label><input type="number" className="inp mono" value={cur.poll_interval_sec || ''} onChange={(e) => set('poll_interval_sec', e.target.value)} /></div>
          <label className="text-sm flex gap-2 items-center cursor-pointer pb-2">
            <input type="checkbox" checked={!!cur.auto_disable} onChange={(e) => set('auto_disable', e.target.checked)} className="w-4 h-4 accent-[#2f665c]" />
            Отключать при лимите
          </label>
        </div>
        <div><label className="lbl">Автоудаление ключей, дней после лимита/срока (0 — выкл)</label><input type="number" min="0" className="inp mono" value={cur.auto_delete_days || 0} onChange={(e) => set('auto_delete_days', e.target.value)} /></div>
        <Btn kind="primary" onClick={save}>Сохранить</Btn>
      </motion.div>

      <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.08 }} className="card p-6 space-y-4">
        <div className="font-medium">Доступ к панели</div>
        <div><label className="lbl">Старый пароль</label><input type="password" className="inp" value={oldPw} onChange={(e) => setOldPw(e.target.value)} /></div>
        <div><label className="lbl">Новый пароль, мин. 6 символов</label><input type="password" className="inp" value={newPw} onChange={(e) => setNewPw(e.target.value)} /></div>
        <Btn onClick={changePw}>Сменить пароль</Btn>
        <div className="text-[13px] text-muted leading-relaxed border-t border-line pt-4">
          Панель и openflux живут на одном сервере — панель запускает <span className="mono">openflux --role=exit</span> локально.
          Для <b>l3</b> нужен root и правило <span className="mono">iptables -A OUTPUT -p tcp --tcp-flags RST RST -s &lt;egress-ip&gt; -j DROP</span>.
          В фаерволе откройте диапазон direct-портов и порт панели.
        </div>
      </motion.div>
    </div>
  );
}
