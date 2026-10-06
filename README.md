# ⚡ OpenFlux Panel

Полноценная панель управления exit-нодами **OpenFlux** — совместимая с официальными клиентами
[**OpenFluxAndroid**](https://github.com/p1neappleXpress/OpenFluxAndroid) и Desktop.
Бэкенд — один бинарь (Go, только стандартная библиотека), фронт — React + Motion,
спокойный светлый дизайн.

## Что умеет

- **Все транспорты ядра**: `direct`, `yandex`, `vyandex`, `boards`, `mailru`, `cupsonline`, `oneme`
  (MAX; токен в ссылку не кладётся — так же как в ядре: `not_shareable`).
- **Ссылки `openflux://v1/…` байт-в-байт как в ядре**: `base64url_nopad(raw_DEFLATE(JSON))`,
  те же поля, те же коды ошибок, та же толерантность к пробелам/padding.
  Проверено: собранное ядро (`--parse-link`) читает ссылки панели один в один.
- **Режимы exit**: `l3` (raw SNAT, Linux+root, быстрее), `l4` (gVisor proxy, везде),
  `stream` (PHP-нода без сервера, носители `cupsonline`/`mailru`, без секрета).
- **Учёт трафика**: опрос IPC-статуса каждого процесса (`bytes_in/bytes_out`), график за 7 дней,
  счётчики ↑/↓ по каждому ключу.
- **Лимиты на ключ**: трафик (ГБ), срок действия, лимит IP; авто-отключение при превышении,
  опциональное **автоудаление** ключей спустя N дней после лимита/срока (настройка, 0 — выкл),
  статусы `active/disabled/limited/expired/error`.
- **Мульти-транспорт с failover**: приоритеты (`direct:100,yandex:50`), активный носитель
  виден в дашборде, контекст шифрования — по правилу ядра.
- **Диагностика**: логи каждого exit-процесса, разбор чужих ссылок, онлайн-IP по direct-портам.

## ⚠️ Архитектура (честно)

Ядро держит **одного клиента на exit-процесс**: второй клиент с тем же секретом вытесняет
первого. Поэтому панель поднимает **отдельный `openflux --role=exit` процесс на ключ**
(свой порт, секрет, IPC-сокет, cookie-store). Отсюда два ограничения:

1. **Не давайте двум ключам один документ/room** — два процесса поделят один носитель.
2. **Лимит IP работает только для `direct`** — у документов IP видит Яндекс, а не exit.

## Установка одной командой

```bash
curl -fsSL https://raw.githubusercontent.com/kfwle/openflux-panel/main/setup.sh | bash
```

Скрипт **спросит**: порт панели, логин/пароль админа (или сгенерирует), белый IP сервера,
диапазон direct-портов, режим по умолчанию, куда ставить, ставить ли ядро, открывать ли
фаервол. Дальше всё сам: поставит Go, склонирует и соберёт панель, **скачает готовое ядро
из последнего node-релиза апстрима (с проверкой SHA256)** — компиляция gVisor нужна только
как запасной вариант, — пропишет systemd, откроет порты, для `l3` добавит правило против
kernel RST, запустит сервис.

Неинтерактивно:

```bash
curl -fsSL https://raw.githubusercontent.com/kfwle/openflux-panel/main/setup.sh | bash -s -- \
  --port 8080 --user admin --mode l4 --yes
# ещё флаги: --pass, --host, --from, --to, --dir, --repo, --no-core, --no-fw
```

Первый вход — ваш логин/пароль → смените пароль в Настройках после входа.

### Для режима l3 (Linux + root)

Установщик добавляет сам, вручную так:

```bash
sudo iptables -A OUTPUT -p tcp --tcp-flags RST RST -s <egress-ip> -j DROP
```

### Docker

```bash
cp /path/to/openflux ./openflux
docker compose up -d --build
```

## Как создать ключ

1. **Ключи → Новый ключ**: имя, режим, секрет (кнопка с кубиком — сгенерировать), транспорты (тип + URL + приоритет).
   Минимум — один `direct` (порт выдастся автоматически).
2. Пример мульти: `direct:100` + `yandex:50` (URL **своего** Яндекс.Документа).
3. Лимиты: трафик, IP, дата «до».
4. QR на строке ключа → сканируйте в OpenFluxAndroid.

## Формат ссылок

```
openflux://v1/<base64url_nopad( raw_DEFLATE( JSON ) )>
```

- `negotiate` обязателен при >1 транспорта и всегда для `direct`.
- `secret` ≥ 16 символов (счёт UTF-16, как в клиентах).
- `stream`: `{"mode":"stream","transports":[{"type":"cupsonline","url":"..."}]}` — без секрета.
- `oneme` в ссылку не входит: в панели храните `token|uid` в поле URL.

## Разработка

```bash
git clone https://github.com/kfwle/openflux-panel.git && cd openflux-panel
go build -o openflux-panel . && ./openflux-panel -port 8080 -data ./data
```

```
openflux-panel/
  main.go manager.go store.go oflink.go auth.go  — бэкенд (stdlib only)
  frontend/   — React + framer-motion + Tailwind (Vite)
  web/        — собранный фронт (коммитится, чтобы не требовать node на сервере)
  setup.sh install.sh Dockerfile docker-compose.yml
```

Пересобрать фронт: `cd frontend && npm i && npm run build` (положит в `../web`).
Для локальной разработки фронта: `npm run dev` (прокси `/api` → `localhost:8080`).

## API

| Метод | Endpoint | Описание |
|---|---|---|
| POST | `/api/login` `{user,pass}` | вход (cookie `of_session`) |
| GET | `/api/stats` | сводка |
| GET/POST | `/api/keys` | список со ссылками / создать |
| PUT/DELETE | `/api/keys/:id` | изменить / удалить |
| POST | `/api/keys/:id/restart`, `/reset-traffic` | рестарт / сброс |
| GET | `/api/keys/:id/link`, `/logs` | ссылка / лог |
| POST | `/api/parse-link` `{link}` | разобрать ссылку |
| GET/PUT | `/api/settings` | настройки |

Лицензия панели: MIT. Ядро OpenFlux и клиенты — их собственные лицензии (GPL-3.0).
