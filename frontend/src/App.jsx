import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { LayoutDashboard, KeyRound, Settings as SettingsIcon, BookOpen, Zap, LogOut } from 'lucide-react';
import { api } from './api';
import Dashboard from './pages/Dashboard';
import Keys from './pages/Keys';
import Settings from './pages/Settings';
import Guide from './pages/Guide';
import { Btn } from './components';

const NAV = [
  { id: 'dash', label: 'Дашборд', icon: LayoutDashboard },
  { id: 'keys', label: 'Ключи', icon: KeyRound },
  { id: 'settings', label: 'Настройки', icon: SettingsIcon },
  { id: 'guide', label: 'Справка', icon: BookOpen },
];

const TITLES = {
  dash: ['Дашборд', 'Состояние процессов, трафик, онлайн'],
  keys: ['Ключи', 'Каждый ключ — отдельный exit-процесс ядра'],
  settings: ['Настройки', 'Exit-нода, сеть и доступ'],
  guide: ['Транспорты и ссылки', 'Совместимо с OpenFluxAndroid'],
};

export default function App() {
  const [me, setMe] = useState(null);
  const [page, setPage] = useState('dash');
  const [keys, setKeys] = useState([]);
  const [stats, setStats] = useState({});
  const [history, setHistory] = useState([]);
  const [settings, setSettings] = useState(null);
  const [toast, setToast] = useState(null);
  const [newKeySignal, setNewKeySignal] = useState(0);

  const notify = (msg) => {
    setToast(msg);
    setTimeout(() => setToast(null), 3200);
  };

  const reload = async () => {
    try {
      const [k, s, h, st] = await Promise.all([
        api('GET', '/api/keys'),
        api('GET', '/api/stats'),
        api('GET', '/api/history'),
        api('GET', '/api/settings'),
      ]);
      setKeys(k); setStats(s); setHistory(h); setSettings(st);
    } catch (e) {
      if (e.code === 'auth') setMe(null);
    }
  };

  useEffect(() => {
    api('GET', '/api/me').then((m) => setMe(m)).catch(() => setMe(null));
  }, []);

  useEffect(() => {
    if (!me) return;
    reload();
    const t = setInterval(reload, 10000);
    return () => clearInterval(t);
  }, [me]);

  if (!me) return <Login onOk={(m) => { setMe(m); }} />;

  const [title, sub] = TITLES[page];

  return (
    <div className="min-h-screen flex">
      {/* sidebar */}
      <aside className="w-60 shrink-0 hidden md:flex flex-col p-5 border-r border-line bg-card/60 sticky top-0 h-screen">
        <div className="flex items-center gap-3 px-1 pb-6">
          <div className="w-9 h-9 rounded-xl bg-pine text-white flex items-center justify-center"><Zap size={17} /></div>
          <div>
            <div className="font-semibold tracking-tight leading-none">OpenFlux</div>
            <div className="text-xs text-faint mt-1">panel · v{me.version}</div>
          </div>
        </div>
        <nav className="space-y-1">
          {NAV.map(({ id, label, icon: Icon }) => (
            <button
              key={id} onClick={() => setPage(id)}
              className={`relative w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm transition ${page === id ? 'text-ink font-medium' : 'text-muted hover:text-ink hover:bg-paper'}`}
            >
              {page === id && (
                <motion.span layoutId="navbg" className="absolute inset-0 bg-pine-soft rounded-xl" transition={{ type: 'spring', stiffness: 400, damping: 34 }} />
              )}
              <Icon size={16} className="relative" />
              <span className="relative">{label}</span>
            </button>
          ))}
        </nav>
        <div className="mt-auto pt-4 border-t border-line">
          <div className="text-[13px] text-muted px-1 mb-2.5">{me.user}</div>
          <button
            onClick={() => api('POST', '/api/logout').finally(() => setMe(null))}
            className="w-full flex items-center gap-2 px-3.5 py-2 rounded-xl text-sm text-muted hover:text-brick hover:bg-brick-soft transition"
          ><LogOut size={15} /> Выйти</button>
        </div>
      </aside>

      {/* main */}
      <div className="flex-1 min-w-0">
        <div className="md:hidden sticky top-0 z-10 bg-paper/90 border-b border-line px-3 py-2.5 flex gap-1.5 overflow-x-auto" style={{ backdropFilter: 'blur(8px)' }}>
          {NAV.map(({ id, label, icon: Icon }) => (
            <button key={id} onClick={() => setPage(id)} className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[13px] whitespace-nowrap ${page === id ? 'bg-pine-soft text-pine font-medium' : 'text-muted'}`}>
              <Icon size={14} />{label}
            </button>
          ))}
        </div>
        <div className="p-5 md:p-8 max-w-6xl mx-auto">
          <AnimatePresence mode="wait">
            <motion.div key={page} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -6 }} transition={{ duration: 0.22 }}>
              <div className="mb-6">
                <h1 className="text-[22px] font-semibold tracking-tight">{title}</h1>
                <p className="text-sm text-muted mt-0.5">{sub}</p>
              </div>
              {page === 'dash' && <Dashboard stats={stats} keys={keys} history={history} goKeys={() => setPage('keys')} onNew={() => { setPage('keys'); setNewKeySignal((n) => n + 1); }} />}
              {page === 'keys' && <Keys keys={keys} reload={reload} notify={notify} openSignal={newKeySignal} />}
              {page === 'settings' && <Settings settings={settings} reload={reload} notify={notify} />}
              {page === 'guide' && <Guide />}
            </motion.div>
          </AnimatePresence>
        </div>
      </div>

      {/* toast */}
      <AnimatePresence>
        {toast && (
          <motion.div
            initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 8 }}
            className="fixed bottom-6 left-1/2 -translate-x-1/2 z-[60] bg-ink text-white text-sm px-4 py-2.5 rounded-xl shadow-lg"
          >{toast}</motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function Login({ onOk }) {
  const [user, setUser] = useState('admin');
  const [pass, setPass] = useState('');
  const [err, setErr] = useState('');
  const go = async () => {
    try {
      await api('POST', '/api/login', { user, pass });
      const m = await api('GET', '/api/me');
      onOk(m);
    } catch (e) { setErr(e.message); }
  };
  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <motion.div
        initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.45 }}
        className="card p-8 w-full max-w-sm" style={{ boxShadow: '0 16px 48px rgba(41,37,36,.08)' }}
      >
        <div className="flex items-center gap-3 mb-7">
          <div className="w-10 h-10 rounded-xl bg-pine text-white flex items-center justify-center"><Zap size={18} /></div>
          <div>
            <div className="font-semibold tracking-tight">OpenFlux Panel</div>
            <div className="text-xs text-muted">управление exit-нодами</div>
          </div>
        </div>
        <div className="space-y-3.5">
          <div><label className="lbl">Логин</label><input className="inp" value={user} onChange={(e) => setUser(e.target.value)} /></div>
          <div><label className="lbl">Пароль</label><input type="password" className="inp" value={pass} onChange={(e) => setPass(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && go()} placeholder="••••••••" /></div>
          {err && <div className="text-sm text-brick">{err}</div>}
          <Btn kind="primary" className="w-full justify-center !py-2.5" onClick={go}>Войти</Btn>
          <div className="text-xs text-faint text-center">Первый вход: admin / admin123</div>
        </div>
      </motion.div>
    </div>
  );
}
