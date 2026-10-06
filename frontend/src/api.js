// Tiny fetch wrapper. Throws Error(message) on non-2xx.
// 401 -> throws {code:'auth'} so App can show login.
export async function api(method, url, body) {
  const r = await fetch(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  const j = await r.json().catch(() => ({ error: 'bad response' }));
  if (r.status === 401) {
    const e = new Error('unauthorized');
    e.code = 'auth';
    throw e;
  }
  if (!r.ok) throw new Error(j.error || `HTTP ${r.status}`);
  return j;
}

export const GB = 1024 ** 3;
export const MB = 1024 ** 2;

export function fmtBytes(b) {
  b = Number(b) || 0;
  if (b < 1024) return `${b} Б`;
  if (b < MB) return `${(b / 1024).toFixed(1)} КБ`;
  if (b < GB) return `${(b / MB).toFixed(1)} МБ`;
  return `${(b / GB).toFixed(2)} ГБ`;
}

export function randomSecret() {
  const a = new Uint8Array(16);
  crypto.getRandomValues(a);
  return [...a].map((x) => x.toString(16).padStart(2, '0')).join('');
}

export const TRANSPORTS = ['direct', 'yandex', 'vyandex', 'boards', 'mailru', 'cupsonline', 'oneme'];
export const TR_HINT = {
  direct: 'порт берётся из поля «Direct-порт»',
  yandex: 'https://docs.yandex.ru/…',
  vyandex: 'https://docs.yandex.ru/… (+ cookies.txt на сервере)',
  boards: 'URL Yandex Board',
  mailru: 'https://cloud.mail.ru/public/…',
  cupsonline: 'room-URL или пусто — exit создаст сам',
  oneme: 'в поле URL: token|uid (в ссылку не попадёт)',
};
