# Codex Session Sync

Скрипт `scripts/codex-sync.py` позволяет переносить локальные Codex-сессии между компьютерами через Git.

Основная цель — сохранить не только код проекта, но и контекст Codex-чата, чтобы на другом компьютере можно было продолжить ту же сессию.

## Что синхронизируется

Скрипт сохраняет rollout-файлы Codex из:

```text
~/.codex/sessions/
```

в каталог проекта:

```text
.codex-sync/
```

Структура выглядит примерно так:

```text
.codex-sync/
├── manifest.json
└── sessions/
    └── 2026/
        └── 08/
            └── 25/
                └── rollout-....jsonl
```

`manifest.json` содержит:

- ID Codex-сессии;
- дату экспорта;
- исходный путь проекта;
- список rollout-файлов;
- SHA256 файлов.

## Важно

Rollout JSONL содержит историю Codex-сессии: сообщения, tool calls, команды, результаты выполнения и часть рабочего контекста.

Поэтому:

- используйте скрипт только в приватных репозиториях;
- не публикуйте `.codex-sync` в публичный Git;
- не редактируйте rollout-файлы вручную;
- не храните в Codex-сессии секреты, API keys и пароли.

## Требования

Нужны:

- Python 3;
- Git;
- Codex CLI;
- SQLite;
- авторизация в Codex.

Проверка:

```bash
python3 --version
git --version
codex --version
sqlite3 --version
```

## Команды

### export

Экспортирует последнюю Codex-сессию текущего Git-репозитория:

```bash
./scripts/codex-sync.py export
```

Пример:

```text
Codex session exported
Session: 01a0302f-74c1-7f73-a8ac-9fa9e1c0ce36
Files:   2
  2026/08/25/rollout-....jsonl
  2026/08/23/rollout-....jsonl
```

### import

Импортирует сохранённую сессию обратно в локальный каталог:

```text
~/.codex/sessions/
```

и регистрирует её в локальной Codex Desktop SQLite-базе.

```bash
./scripts/codex-sync.py import
```

При импорте скрипт:

1. копирует rollout-файлы;
2. создаёт backup локального rollout при конфликте;
3. находит или создаёт локальный Codex Project;
4. обновляет `cwd`;
5. привязывает thread к текущему проекту;
6. обновляет recency для отображения чата в UI.

### resume

Импортирует сессию и сразу запускает:

```bash
codex resume <SESSION_ID>
```

Использование:

```bash
./scripts/codex-sync.py resume
```

### status

Показывает сохранённую сессию:

```bash
./scripts/codex-sync.py status
```

Пример:

```text
Session:  01a0302f-74c1-7f73-a8ac-9fa9e1c0ce36
Exported: 2026-08-31T18:00:00+00:00
From:     C:\Users\user\project
Files:    2
```

### list

Показывает локальные Codex-сессии:

```bash
./scripts/codex-sync.py list
```

Полезно для диагностики.

### push

Основная команда на компьютере, где работа завершена:

```bash
./scripts/codex-sync.py push
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

Если изменений нет, новый commit не создаётся.

### pull

Основная команда на другом компьютере:

```bash
./scripts/codex-sync.py pull
```

Она выполняет:

```text
git pull --rebase
↓
import Codex session
↓
register thread in Codex Desktop
↓
codex resume <SESSION_ID>
```

## Обычный workflow

### Компьютер A

Работаем с проектом и Codex.

Перед переходом на другой компьютер:

```bash
./scripts/codex-sync.py push
```

### Компьютер B

Получаем изменения:

```bash
git clone <repo>
cd <repo>
```

При первом запуске:

```bash
codex login
```

Затем:

```bash
./scripts/codex-sync.py pull
```

После этого должна открыться та же Codex-сессия с сохранённой историей.

В Codex Desktop thread также должен появиться в списке проекта.

## `.gitattributes`

Чтобы Git не пытался строить огромные diff для JSONL-файлов, добавьте:

```gitattributes
.codex-sync/**/*.jsonl -diff
```

Можно выполнить:

```bash
echo '.codex-sync/**/*.jsonl -diff' >> .gitattributes
```

## Windows

На Windows скрипт поддерживает:

- Git Bash;
- пути с кириллицей;
- OneDrive;
- пути вида `C:\...`;
- Codex canonical paths с префиксом `\\?\`.

Пример:

```text
C:\Users\user\OneDrive\Рабочий стол\Code\project
```

## macOS / Linux

Codex home по умолчанию:

```text
~/.codex
```

SQLite:

```text
~/.codex/state_5.sqlite
```

Rollout-файлы:

```text
~/.codex/sessions/
```

## Codex Desktop UI

Простого копирования rollout-файла недостаточно для появления чата в Desktop UI.

Скрипт дополнительно обновляет:

```text
~/.codex/state_5.sqlite
```

и связывает thread с локальным Project через:

```text
threads.project_id
```

При переносе между компьютерами `cwd` автоматически заменяется на путь текущего Git-репозитория.

Например:

```text
Windows:
C:\Users\user\OneDrive\Рабочий стол\Code\daria app

macOS:
/Users/user/Desktop/daria-app
```

При этом ID Codex-сессии остаётся тем же.

## Возможные проблемы

### No Codex sessions found for repository

Пример:

```text
codex-sync: No Codex sessions found for repository
```

Проверьте:

```bash
./scripts/codex-sync.py list
```

и убедитесь, что `CWD` совпадает с текущим Git-репозиторием.

### Чат не появился в Codex Desktop

Проверьте наличие thread:

```bash
sqlite3 ~/.codex/state_5.sqlite "
SELECT id,title,cwd,project_id,archived
FROM threads
WHERE id='<SESSION_ID>';
"
```

И наличие проекта:

```bash
sqlite3 ~/.codex/state_5.sqlite "
SELECT * FROM projects;
"
```

После изменения SQLite полностью перезапустите Codex Desktop / VS Code.

### Session уже открыта на другом компьютере

Не рекомендуется одновременно продолжать одну и ту же Codex-сессию на двух компьютерах.

Лучший workflow:

```text
Компьютер A:
push → закрыть Codex

Компьютер B:
pull → продолжить работу
```


## Makefile

Для удобства в корень проекта можно добавить `Makefile`:

```makefile
.PHONY: codex-export codex-import codex-resume codex-status codex-list codex-push codex-pull

CODEX_SYNC := ./scripts/codex-sync.py

codex-export:
	$(CODEX_SYNC) export

codex-import:
	$(CODEX_SYNC) import

codex-resume:
	$(CODEX_SYNC) resume

codex-status:
	$(CODEX_SYNC) status

codex-list:
	$(CODEX_SYNC) list

codex-push:
	$(CODEX_SYNC) push

codex-pull:
	$(CODEX_SYNC) pull
```

После этого вместо прямого запуска Python-скрипта можно использовать:

```bash
make codex-export
make codex-import
make codex-resume
make codex-status
make codex-list
make codex-push
make codex-pull
```

Основной ежедневный workflow становится таким.

На компьютере, где закончили работу:

```bash
make codex-push
```

На другом компьютере:

```bash
make codex-pull
```

`make codex-push` выполняет:

```text
export Codex session
↓
git add -A
↓
git commit
↓
git push
```

`make codex-pull` выполняет:

```text
git pull --rebase
↓
import Codex session
↓
register thread in Codex Desktop
↓
codex resume <SESSION_ID>
```

На Windows команды `make ...` требуют установленный `make`. Если его нет, можно продолжать использовать скрипт напрямую:

```bash
./scripts/codex-sync.py push
./scripts/codex-sync.py pull
```


## Рекомендуемые alias

Для удобства можно добавить:

```bash
alias cxpush='./scripts/codex-sync.py push'
alias cxpull='./scripts/codex-sync.py pull'
```

Тогда:

```bash
cxpush
```

и:

```bash
cxpull
```

## Ограничения

Это не официальный механизм cloud-sync Codex.

Скрипт работает с внутренними локальными файлами Codex:

```text
~/.codex/sessions/
~/.codex/state_5.sqlite
```

Их формат может измениться в будущих версиях Codex.

Если после обновления Codex скрипт перестал работать, первым делом проверьте:

```bash
sqlite3 ~/.codex/state_5.sqlite ".schema threads"
```

и:

```bash
sqlite3 ~/.codex/state_5.sqlite ".schema projects"
```
