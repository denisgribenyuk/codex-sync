# codex-sync

`codex-sync` — набор утилит для переноса локальных Codex-сессий между компьютерами через Git.

Репозиторий содержит две реализации одной и той же идеи:

- **Go** — самостоятельный бинарник, удобный для постоянного использования и переноса между машинами;
- **Python** — исходная версия скрипта, удобная для чтения, отладки и быстрых изменений.

Обе версии работают с локальными данными Codex, экспортируют историю thread в `.codex-sync/`, а затем позволяют импортировать её на другом компьютере и продолжить работу с сохранённым контекстом.

---

## Что решает этот репозиторий

Codex хранит локальные сессии на конкретном компьютере.

Если проект клонировать на другую машину, код будет доступен через Git, но история локального Codex-чата сама по себе не перенесётся.

`codex-sync` добавляет недостающий слой синхронизации:

```text
Компьютер A
    │
    │ codex-sync push
    ▼
Git repository
    │
    │ git pull + import
    ▼
Компьютер B
    │
    │ codex-sync pull
    ▼
тот же Codex thread
```

Синхронизируются:

- rollout JSONL текущего Codex thread;
- `session_id`;
- SHA256 файлов;
- основные метаданные thread;
- привязка thread к локальному Codex Project;
- локальный `cwd` для нового компьютера.

---

## Структура репозитория

```text
codex-sync/
├── README.md
├── go/
│   ├── dist/
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── Makefile
│   └── README-go-codex-sync.md
│
└── python/
    ├── __pycache__/
    ├── codex-sync.py
    ├── Makefile
    └── README-codex-sync.md
```

---

# Реализации

## Go

Каталог:

```text
go/
```

Go-версия предназначена для повседневного использования.

Плюсы:

- один бинарник;
- Python на целевой машине не нужен;
- SQLite CLI не нужен;
- удобно положить бинарник в `PATH`;
- легко собрать под Windows, macOS и Linux;
- подходит для переноса между несколькими устройствами.

Документация:

```text
go/README-go-codex-sync.md
```

Быстрая сборка:

```bash
cd go
go mod tidy
go build -o codex-sync .
```

После этого:

```bash
./codex-sync status
```

Или установить в `PATH`:

```bash
sudo cp codex-sync /usr/local/bin/codex-sync
```

После установки:

```bash
codex-sync push
codex-sync pull
```

---

## Python

Каталог:

```text
python/
```

Python-версия — исходная реализация.

Она полезна, если нужно:

- быстро изменить логику;
- отладить поведение;
- посмотреть алгоритм в более компактном виде;
- использовать скрипт без сборки Go-бинарника.

Документация:

```text
python/README-codex-sync.md
```

Запуск:

```bash
cd python
chmod +x codex-sync.py
./codex-sync.py status
```

Или:

```bash
python3 codex-sync.py status
```

---

# Основные команды

Обе реализации поддерживают одинаковый набор команд.

## `export`

Экспортирует наиболее свежую Codex-сессию текущего Git-проекта:

```bash
codex-sync export
```

или для Python:

```bash
./codex-sync.py export
```

---

## `import`

Импортирует сохранённую сессию в локальный Codex:

```bash
codex-sync import
```

---

## `resume`

Импортирует сессию и сразу запускает:

```text
codex resume <SESSION_ID>
```

Команда:

```bash
codex-sync resume
```

---

## `status`

Показывает информацию о сохранённой сессии:

```bash
codex-sync status
```

---

## `list`

Показывает локальные Codex-сессии:

```bash
codex-sync list
```

---

## `push`

Основная команда на компьютере, где работа закончена:

```bash
codex-sync push
```

Она выполняет:

```text
export
↓
git add -A
↓
git commit
↓
git push
```

---

## `pull`

Основная команда на другом компьютере:

```bash
codex-sync pull
```

Она выполняет:

```text
git pull --rebase
↓
import
↓
register thread in Codex Desktop
↓
codex resume <SESSION_ID>
```

---

# Рекомендуемый workflow

## Компьютер A

Работаем с проектом и Codex.

Перед переходом на другой компьютер:

```bash
codex-sync push
```

После этого желательно закрыть текущий Codex thread на этой машине.

## Компьютер B

Если репозиторий ещё не клонирован:

```bash
git clone <repo>
cd <repo>
```

При необходимости:

```bash
codex login
```

Затем:

```bash
codex-sync pull
```

После этого:

- подтянется код;
- импортируется история thread;
- thread будет зарегистрирован в локальной Codex базе;
- `cwd` будет заменён на путь текущего checkout;
- чат должен появиться в Codex Desktop;
- будет запущен `codex resume`.

---

# Какую реализацию использовать

Для обычной работы рекомендуется **Go-версия**.

Используйте Go, если нужен:

```text
один бинарник
+
минимум зависимостей
+
работа на нескольких машинах
```

Python-версию имеет смысл оставить в репозитории как:

- reference implementation;
- удобный вариант для отладки;
- запасной инструмент;
- более простой код для быстрых экспериментов.

Практически:

```text
Go     → использовать
Python → хранить и поддерживать как fallback/reference
```

---

# Где Codex хранит данные

По умолчанию:

```text
~/.codex/
```

или в каталоге, указанном через:

```text
CODEX_HOME
```

Rollout-файлы:

```text
~/.codex/sessions/
```

Локальная база Codex Desktop:

```text
~/.codex/state_5.sqlite
```

`codex-sync` не копирует весь каталог `.codex`.

---

# `.codex-sync`

В синхронизируемом проекте после `export` появляется:

```text
.codex-sync/
├── manifest.json
└── sessions/
    └── ...
```

В `manifest.json` сохраняются:

- ID thread;
- исходный путь проекта;
- время экспорта;
- список rollout-файлов;
- SHA256;
- метаданные thread.

---

# Codex Desktop UI

Одного копирования rollout JSONL недостаточно, чтобы перенесённый thread появился в UI.

Поэтому утилиты дополнительно работают с:

```text
~/.codex/state_5.sqlite
```

и таблицами:

```text
threads
projects
```

При импорте:

- создаётся локальный Project, если его ещё нет;
- thread получает `project_id`;
- `cwd` заменяется на текущий Git root;
- `rollout_path` заменяется на локальный путь;
- `archived` устанавливается в `0`;
- обновляются `updated_at` и `recency_at`.

---

# Поддерживаемые платформы

Обе реализации рассчитаны на:

- Windows;
- macOS;
- Linux.

Go-версия может собираться отдельно под:

```text
linux/amd64
linux/arm64
darwin/amd64
darwin/arm64
windows/amd64
windows/arm64
```

---

# Windows и OneDrive

Поддерживаются пути вида:

```text
C:\Users\user\OneDrive\Рабочий стол\Code\project
```

Учитываются:

- кириллица;
- `\` и `/`;
- case-insensitive пути;
- canonical paths с префиксом `\\?\`.

---

# `.gitattributes`

Rollout JSONL могут быть большими, поэтому рекомендуется добавить в синхронизируемые проекты:

```gitattributes
.codex-sync/**/*.jsonl -diff
```

Например:

```bash
echo '.codex-sync/**/*.jsonl -diff' >> .gitattributes
```

---

# Безопасность

Rollout-файлы содержат историю Codex-сессии.

Там могут быть:

- сообщения пользователя;
- ответы модели;
- команды терминала;
- результаты tool calls;
- фрагменты файлов;
- рабочий контекст проекта.

Поэтому:

- используйте `.codex-sync` только в приватных репозиториях;
- не публикуйте rollout-файлы;
- не передавайте их третьим лицам;
- не вставляйте секреты и API keys в Codex-чаты.

Не синхронизируются:

```text
~/.codex/auth.json
~/.codex/config.toml
```

---

# Ограничения

Это не официальный механизм cloud-sync Codex.

Утилиты работают с внутренними локальными структурами:

```text
~/.codex/sessions/
~/.codex/state_5.sqlite
```

Их формат может измениться в будущих версиях Codex.

Если после обновления Codex синхронизация перестала работать, в первую очередь проверьте:

```bash
sqlite3 ~/.codex/state_5.sqlite ".schema threads"
```

и:

```bash
sqlite3 ~/.codex/state_5.sqlite ".schema projects"
```

---

# Документация

Подробности по каждой реализации:

- [Go implementation](go/README-go-codex-sync.md)
- [Python implementation](python/README-codex-sync.md)
