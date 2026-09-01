# codex-sync

`codex-sync` — CLI-утилита на Go для переноса локальных Codex-сессий между компьютерами через Git.

Она синхронизирует не только код проекта, но и историю Codex-чата, чтобы на другом компьютере можно было продолжить тот же thread.

## Возможности

Утилита умеет:

- находить Codex-сессии текущего Git-репозитория;
- экспортировать rollout JSONL из `~/.codex/sessions`;
- сохранять `session_id`, список файлов и SHA256 в `.codex-sync/manifest.json`;
- переносить несколько rollout-файлов одного thread;
- импортировать сессию на другом компьютере;
- создавать backup rollout-файлов при конфликте;
- регистрировать thread в `~/.codex/state_5.sqlite`;
- создавать локальный Codex Project;
- связывать thread с Project через `project_id`;
- обновлять `cwd` под текущий компьютер;
- обновлять recency, чтобы чат отображался в Codex Desktop;
- запускать `codex resume <SESSION_ID>`;
- выполнять Git push/pull вместе с синхронизацией.

Поддерживаются:

- macOS;
- Linux;
- Windows;
- Git Bash;
- пути с кириллицей;
- OneDrive;
- разные пути одного проекта на разных компьютерах.

---

## Требования

Для запуска готового бинарника нужны:

- Git;
- Codex CLI;
- локальная установка Codex / Codex Desktop.

Для сборки дополнительно нужен Go.

Проверка:

```bash
git --version
codex --version
go version
```

SQLite CLI для работы бинарника не требуется.

Утилита работает с SQLite напрямую через Go-драйвер:

```text
modernc.org/sqlite
```

---

## Структура проекта

Пример:

```text
codex-sync/
├── go.mod
├── go.sum
├── main.go
├── Makefile
└── README.md
```

После кроссплатформенной сборки:

```text
dist/
├── codex-sync-linux-amd64
├── codex-sync-linux-arm64
├── codex-sync-darwin-amd64
├── codex-sync-darwin-arm64
├── codex-sync-windows-amd64.exe
└── codex-sync-windows-arm64.exe
```

---

## Инициализация проекта

Если Go module ещё не создан:

```bash
go mod init github.com/USERNAME/codex-sync
```

Добавить SQLite-драйвер:

```bash
go get modernc.org/sqlite
```

После этого:

```bash
go mod tidy
```

---

## Сборка

Для текущей системы:

```bash
go build -o codex-sync .
```

Проверка:

```bash
./codex-sync
```

Должен появиться список команд:

```text
Usage:

  codex-sync export
  codex-sync import
  codex-sync resume
  codex-sync status
  codex-sync list
  codex-sync push
  codex-sync pull
```

---

## Makefile

Пример `Makefile` для сборки под основные платформы:

```makefile
APP := codex-sync
DIST := dist

.PHONY: build clean build-all \
        linux-amd64 linux-arm64 \
        darwin-amd64 darwin-arm64 \
        windows-amd64 windows-arm64

build:
	go build -trimpath -ldflags="-s -w" -o $(APP) .

clean:
	rm -rf $(DIST) $(APP)

build-all: clean \
	linux-amd64 \
	linux-arm64 \
	darwin-amd64 \
	darwin-arm64 \
	windows-amd64 \
	windows-arm64

linux-amd64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-linux-amd64 .

linux-arm64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-linux-arm64 .

darwin-amd64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-darwin-amd64 .

darwin-arm64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-darwin-arm64 .

windows-amd64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-windows-amd64.exe .

windows-arm64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-windows-arm64.exe .
```

Собрать всё:

```bash
make build-all
```

---

## Установка бинарника в PATH

### macOS / Linux

Например:

```bash
sudo cp codex-sync /usr/local/bin/codex-sync
```

Проверка:

```bash
which codex-sync
codex-sync status
```

Можно установить только для текущего пользователя:

```bash
mkdir -p ~/bin
cp codex-sync ~/bin/
```

Добавить в `~/.zshrc` или `~/.bashrc`:

```bash
export PATH="$HOME/bin:$PATH"
```

### Windows

Положите:

```text
codex-sync.exe
```

например в:

```text
C:\Tools\codex-sync\
```

и добавьте этот каталог в `PATH`.

После этого:

```powershell
codex-sync status
```

---

## Где Codex хранит данные

По умолчанию:

```text
~/.codex/
```

или в каталоге из переменной:

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

Утилита не синхронизирует весь `~/.codex`.

В Git попадают только данные выбранной сессии.

---

## Каталог `.codex-sync`

После:

```bash
codex-sync export
```

в текущем Git-проекте появляется:

```text
.codex-sync/
├── manifest.json
└── sessions/
    └── 2026/
        └── 08/
            ├── 23/
            │   └── rollout-....jsonl
            └── 25/
                └── rollout-....jsonl
```

`manifest.json` хранит:

- `session_id`;
- `source_repo`;
- `exported_at`;
- список rollout-файлов;
- SHA256 каждого файла;
- метаданные thread, если они доступны в локальной Codex SQLite-базе.

Пример:

```json
{
  "session_id": "01a0302f-74c1-7f73-a8ac-9fa9e1c0ce36",
  "source_repo": "/Users/user/Desktop/project",
  "exported_at": "2026-09-01T12:00:00Z",
  "files": [
    {
      "relative_path": "2026/08/25/rollout-....jsonl",
      "sha256": "..."
    }
  ],
  "thread": {
    "title": "Внеси изменения в брендинг приложения.",
    "preview": "Внеси изменения в брендинг приложения.",
    "source": "vscode",
    "model_provider": "openai"
  }
}
```

---

## Команды

### `export`

Экспортирует наиболее свежий Codex thread текущего Git-проекта:

```bash
codex-sync export
```

Пример:

```text
Codex session exported
Session: 01a0302f-74c1-7f73-a8ac-9fa9e1c0ce36
Files:   2
  2026/08/25/rollout-....jsonl
  2026/08/23/rollout-....jsonl
```

---

### `import`

Импортирует сохранённую сессию:

```bash
codex-sync import
```

Утилита:

1. читает `.codex-sync/manifest.json`;
2. копирует rollout-файлы в локальный `~/.codex/sessions`;
3. создаёт backup локального rollout, если содержимое отличается;
4. регистрирует thread в локальной Codex SQLite-базе;
5. создаёт или находит локальный Project;
6. заменяет `cwd` на путь текущего checkout;
7. связывает thread через `project_id`;
8. обновляет recency.

---

### `resume`

Импортирует сессию и сразу продолжает её:

```bash
codex-sync resume
```

Логика:

```text
import
↓
register thread in Codex Desktop
↓
codex resume <SESSION_ID>
```

---

### `status`

Показывает информацию о сохранённой сессии:

```bash
codex-sync status
```

Пример:

```text
Session:  01a0302f-74c1-7f73-a8ac-9fa9e1c0ce36
Exported: 2026-09-01T12:00:00Z
From:     /Users/user/Desktop/project
Files:    2
```

---

### `list`

Показывает локальные Codex-сессии:

```bash
codex-sync list
```

Пример:

```text
Session: 01a0302f-74c1-7f73-a8ac-9fa9e1c0ce36
CWD:     /Users/user/Desktop/project
File:    /Users/user/.codex/sessions/2026/08/25/rollout-....jsonl
```

---

### `push`

Основная команда на компьютере, где работа закончена:

```bash
codex-sync push
```

Выполняет:

```text
export
↓
git add -A
↓
git commit
↓
git push
```

Если изменений нет, новый commit не создаётся.

---

### `pull`

Основная команда на другом компьютере:

```bash
codex-sync pull
```

Выполняет:

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

## Обычный workflow

### Компьютер A

Перед переходом на другой компьютер:

```bash
codex-sync push
```

После этого желательно закрыть текущий Codex thread.

### Компьютер B

Если репозиторий ещё не клонирован:

```bash
git clone <repo>
cd <repo>
```

При необходимости авторизоваться:

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

## `.gitattributes`

Rollout JSONL могут быть большими, поэтому рекомендуется отключить обычный Git diff:

```gitattributes
.codex-sync/**/*.jsonl -diff
```

Добавить:

```bash
echo '.codex-sync/**/*.jsonl -diff' >> .gitattributes
```

---

## Windows

Поддерживаются пути вида:

```text
C:\Users\user\OneDrive\Рабочий стол\Code\project
```

Утилита нормализует:

- `\` и `/`;
- Windows case-insensitive paths;
- canonical paths с префиксом `\\?\`.

Это позволяет переносить один thread между Windows, macOS и Linux, даже если checkout расположен по разным путям.

---

## Codex Desktop UI

Простого копирования rollout JSONL недостаточно, чтобы thread появился в UI.

Поэтому `codex-sync` дополнительно работает с:

```text
~/.codex/state_5.sqlite
```

и таблицами:

```text
threads
projects
```

При импорте:

- создаётся локальный `project`, если его ещё нет;
- thread получает `project_id`;
- `cwd` заменяется на путь текущего Git-репозитория;
- `rollout_path` заменяется на локальный путь;
- `archived` устанавливается в `0`;
- обновляются `updated_at` и `recency_at`.

---

## Troubleshooting

### `No Codex sessions found for repository`

Ошибка:

```text
codex-sync: No Codex sessions found for repository
```

Проверьте:

```bash
codex-sync list
```

И Git root:

```bash
git rev-parse --show-toplevel
```

`CWD` найденной сессии должен соответствовать текущему проекту.

### Чат не появился в Codex Desktop

Проверьте thread:

```bash
sqlite3 ~/.codex/state_5.sqlite "
SELECT id,title,cwd,project_id,archived
FROM threads
WHERE id='<SESSION_ID>';
"
```

Проверьте проекты:

```bash
sqlite3 ~/.codex/state_5.sqlite "
SELECT * FROM projects;
"
```

После ручных изменений SQLite полностью перезапустите Codex Desktop / VS Code.

### `state_5.sqlite` не найден

Если Codex Desktop ещё ни разу не запускался, локальная база может отсутствовать.

В этом случае rollout-файлы импортируются, но регистрация thread в UI будет пропущена.

Запустите Codex Desktop хотя бы один раз и повторите:

```bash
codex-sync import
```

### Конфликт rollout-файла

Если локальный rollout уже существует и отличается от импортируемого, создаётся backup:

```text
rollout-....jsonl.bak-20260901-163000
```

### Session открыта на другом компьютере

Не рекомендуется одновременно продолжать один thread на двух машинах.

Рекомендуемый порядок:

```text
Компьютер A:
codex-sync push
↓
закрыть Codex

Компьютер B:
codex-sync pull
↓
продолжить работу
```

---

## Безопасность

Rollout JSONL содержит историю Codex-сессии.

В ней могут быть:

- пользовательские сообщения;
- ответы модели;
- tool calls;
- команды терминала;
- результаты выполнения;
- фрагменты файлов;
- рабочий контекст проекта.

Поэтому рекомендуется:

- использовать `codex-sync` только с приватными репозиториями;
- не публиковать `.codex-sync` в публичный Git;
- не передавать rollout-файлы третьим лицам;
- не вставлять секреты, пароли и API keys в Codex-чаты.

Не синхронизируются:

```text
~/.codex/auth.json
~/.codex/config.toml
```

---

## Рекомендуемый сценарий

После установки бинарника в `PATH` ежедневная работа сводится к двум командам.

На компьютере, где работа закончена:

```bash
codex-sync push
```

На другом компьютере:

```bash
codex-sync pull
```

---

## Ограничения

Это не официальный механизм cloud-sync Codex.

Утилита работает с внутренними локальными структурами Codex:

```text
~/.codex/sessions/
~/.codex/state_5.sqlite
```

Их формат может измениться в будущих версиях Codex.

После обновления Codex при проблемах в первую очередь проверьте схемы:

```bash
sqlite3 ~/.codex/state_5.sqlite ".schema threads"
```

```bash
sqlite3 ~/.codex/state_5.sqlite ".schema projects"
```

Если структура таблиц изменилась, код регистрации thread может потребовать обновления.
