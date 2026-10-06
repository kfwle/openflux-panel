import { motion } from 'framer-motion';

const ROWS = [
  ['direct', 'порт на exit', 'да · dial=host:port', 'Только с --negotiate. Единственный с лимитом IP. Открыть порт в фаерволе.'],
  ['yandex', 'URL Яндекс.Документа', 'да', 'WebSocket. PoW-капча решается сама.'],
  ['vyandex', 'URL + cookies.txt залогиненного', 'да', 'HTTP relay + WebSocket.'],
  ['boards', 'URL Yandex Board', 'да', 'WebSocket.'],
  ['mailru', 'публичная ссылка cloud.mail.ru', 'да', 'WebSocket. Также носитель stream-режима.'],
  ['cupsonline', 'room-ссылка или пусто', 'да', 'Centrifugo rooms. Носитель stream-режима.'],
  ['oneme', 'MAX token + uid', 'нет — токен личный', 'WebRTC DataChannel. В панели: token|uid в поле URL.'],
];

export default function Guide() {
  return (
    <div className="space-y-4 max-w-4xl">
      <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} className="card p-6">
        <div className="font-medium mb-2">Формат ссылки</div>
        <div className="mono text-xs p-3.5 rounded-xl overflow-x-auto bg-paper border border-line">openflux://v1/&lt;base64url_nopad( raw_DEFLATE( JSON ) )&gt;</div>
        <pre className="mono text-xs mt-3 p-4 rounded-xl overflow-x-auto bg-paper border border-line">{`{
  "name": "client-1",
  "negotiate": true,   // auth-сессия; обязательна для >1 транспорта и direct
  "codec": "batched",  // batched+zstd | legacy (LZ4)
  "secret": "…",       // = --encryption-key-file, AES-256-GCM, мин. 16 символов
  "context": "https://…", // KDF-контекст, обязан совпасть у обеих сторон
  "transports": [
    {"type": "direct", "priority": 100, "dial": "203.0.113.7:20000"},
    {"type": "yandex", "url": "https://docs.yandex.ru/…", "priority": 50}
  ]
}`}</pre>
        <div className="text-[13px] text-muted mt-3">Приоритет выше — трафик идёт туда первым, при обрыве переключается ниже. Контекст у клиента и exit обязан совпасть, иначе handshake не сойдётся.</div>
      </motion.div>

      <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.06 }} className="card overflow-hidden">
        <div className="overflow-x-auto">
          <table className="ledger">
            <thead><tr><th>Транспорт</th><th>Что нужно</th><th>В ссылку</th><th>Заметка</th></tr></thead>
            <tbody>
              {ROWS.map(([t, need, link, note]) => (
                <tr key={t}>
                  <td className="mono font-medium">{t}</td>
                  <td className="text-[13px]">{need}</td>
                  <td className="text-[13px]">{link}</td>
                  <td className="text-[13px] text-muted">{note}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </motion.div>

      <div className="grid md:grid-cols-2 gap-4">
        <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="card p-6">
          <div className="font-medium mb-3">Режимы exit</div>
          <div className="text-sm space-y-2.5 text-ink/80">
            <div><b className="mono">l3</b> — raw SNAT/DNAT, TCP end-to-end, быстрее. Только Linux + root.</div>
            <div><b className="mono">l4</b> — gVisor proxy, двойная терминация TCP. Работает везде без root.</div>
            <div><b className="mono">stream</b> — клиент к PHP-ноде на обычном хостинге, TCP 80/443, без шифра ядра.</div>
          </div>
        </motion.div>
        <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.14 }} className="card p-6">
          <div className="font-medium mb-3">Почему один ключ — один процесс</div>
          <div className="text-sm space-y-2.5 text-ink/80">
            <div>Ядро держит одного клиента на exit-процесс: второй с тем же секретом вытесняет первого. Поэтому панель поднимает отдельный процесс на ключ.</div>
            <div>Не давайте двум ключам один документ или room. Лимит IP работает только для direct.</div>
          </div>
        </motion.div>
      </div>
    </div>
  );
}
