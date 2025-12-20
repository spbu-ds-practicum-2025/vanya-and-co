# Руководство по запуску облачного хранилища

Этот документ описывает различные способы запуска системы облачного хранилища.

## Содержание
1. [Запуск через Docker](#запуск-через-docker)
2. [Запуск в отдельных терминалах](#запуск-в-отдельных-терминалах)
3. [Архитектура и порты](#архитектура-и-порты)
4. [Тестирование изоляции сервисов](#тестирование-изоляции-сервисов)

---

## Запуск через Docker

### Предварительные требования
- Docker версии 20.10 или выше
- Docker Compose версии 1.29 или выше

### Команды запуска

```bash
# Запуск всех сервисов
docker-compose up

# Запуск в фоновом режиме
docker-compose up -d

# Просмотр логов
docker-compose logs -f

# Остановка всех сервисов
docker-compose down

# Остановка с удалением volumes
docker-compose down -v
```

### Запуск отдельных сервисов через Docker

```bash
# Запуск только Auth сервиса
docker-compose up auth

# Запуск только File сервиса
docker-compose up file

# Запуск только Sharing сервиса
docker-compose up sharing

# Запуск только Gateway
docker-compose up gateway
```

### Проверка работоспособности

После запуска откройте в браузере:
- **Главная страница**: http://localhost:8080
- **Health check Gateway**: http://localhost:8080/health
- **Health check Auth**: http://localhost:5100/health

---

## Запуск в отдельных терминалах

### Предварительные требования
- Go версии 1.20 или выше
- Все зависимости установлены (`go mod download`)

### Linux/macOS

#### Вариант 1: Автоматический запуск с tmux

```bash
# Сделать скрипты исполняемыми
chmod +x start-*.sh

# Запустить все сервисы в tmux
./start-all.sh
```

Управление tmux сессией:
- `Ctrl+B, затем 0-3` - переключение между окнами
- `Ctrl+B, затем D` - отключиться от сессии (сервисы продолжат работать)
- `tmux attach -t cloud-storage` - подключиться обратно
- `tmux kill-session -t cloud-storage` - остановить все сервисы

#### Вариант 2: Ручной запуск в отдельных терминалах

Откройте 4 терминала и выполните в каждом:

**Терминал 1 - Auth Service:**
```bash
./start-auth.sh
# или
export HTTP_PORT=5100 GRPC_PORT=5101
go run services/auth/cmd/server/main.go
```

**Терминал 2 - File Service:**
```bash
./start-file.sh
# или
export GRPC_PORT=5200
go run services/file/cmd/server/main.go
```

**Терминал 3 - Sharing Service:**
```bash
./start-sharing.sh
# или
export GRPC_PORT=5300
go run services/sharing/cmd/server/main.go
```

**Терминал 4 - Gateway:**
```bash
# Подождите 2-3 секунды после запуска других сервисов
./start-gateway.sh
# или
export PORT=8080 AUTH_GRPC_ADDR=localhost:5101 FILE_ADDR=localhost:5200 SHARE_ADDR=localhost:5300
go run services/gateway/cmd/server/main.go
```

### Windows

#### Автоматический запуск

Дважды кликните на `start-all.bat` или выполните в командной строке:
```cmd
start-all.bat
```

Это откроет 4 окна командной строки для каждого сервиса.

#### Ручной запуск

Откройте 4 окна командной строки и выполните в каждом:

**Окно 1 - Auth Service:**
```cmd
start-auth.bat
```

**Окно 2 - File Service:**
```cmd
start-file.bat
```

**Окно 3 - Sharing Service:**
```cmd
start-sharing.bat
```

**Окно 4 - Gateway:**
```cmd
start-gateway.bat
```

---

## Архитектура и порты

### Сервисы и их порты

| Сервис | HTTP Port | gRPC Port | Описание |
|--------|-----------|-----------|----------|
| **Auth Service** | 5100 | 5101 | Аутентификация и управление сессиями |
| **File Service** | - | 5200 | Хранение и управление файлами |
| **Sharing Service** | - | 5300 | Управление публичными ссылками |
| **Gateway** | 8080 | - | API Gateway и веб-интерфейс |

### Взаимодействие сервисов

```
Клиент (браузер)
    ↓ HTTP
Gateway (localhost:8080)
    ↓ gRPC
    ├─→ Auth Service (localhost:5101)
    ├─→ File Service (localhost:5200)
    └─→ Sharing Service (localhost:5300)
```

### Структура данных

```
vanya-and-co/
├── services/
│   ├── auth/data/          # База данных пользователей и сессий
│   ├── file/
│   │   ├── storage/        # Хранилище файлов
│   │   └── data/           # Метаданные файлов
│   └── sharing/data/       # База данных ссылок
```

---

## Тестирование изоляции сервисов

### Сценарий 1: Отключение Auth Service

1. Запустите все сервисы
2. Остановите Auth Service (Ctrl+C в терминале или `docker-compose stop auth`)
3. Попробуйте:
   - ✅ Открыть главную страницу (localhost:8080) - работает
   - ❌ Войти в систему - не работает (сервис недоступен)
   - ❌ Загрузить файл - не работает (требуется авторизация)

**Ожидаемое поведение**: Gateway продолжает работать, но операции требующие авторизации недоступны.

### Сценарий 2: Отключение File Service

1. Запустите все сервисы и войдите в систему
2. Остановите File Service
3. Попробуйте:
   - ✅ Открыть dashboard - работает
   - ✅ Выйти из системы - работает
   - ❌ Загрузить файл - не работает
   - ❌ Просмотреть список файлов - не работает

**Ожидаемое поведение**: Авторизация работает, но операции с файлами недоступны.

### Сценарий 3: Отключение Sharing Service

1. Запустите все сервисы
2. Остановите Sharing Service
3. Попробуйте:
   - ✅ Войти в систему - работает
   - ✅ Загрузить файл - работает
   - ✅ Скачать файл - работает
   - ❌ Создать публичную ссылку - не работает

**Ожидаемое поведение**: Основные операции работают, но функции обмена недоступны.

### Сценарий 4: Отключение Gateway

1. Запустите все сервисы
2. Остановите Gateway
3. Попробуйте:
   - ❌ Открыть localhost:8080 - не работает
   - ✅ Сервисы Auth, File, Sharing продолжают работать на своих портах

**Ожидаемое поведение**: Веб-интерфейс недоступен, но backend сервисы работают.

---

## Проверка работоспособности

### Health Check endpoints

```bash
# Gateway
curl http://localhost:8080/health

# Auth Service
curl http://localhost:5100/health
```

### Проверка gRPC сервисов

Для проверки gRPC сервисов можно использовать grpcurl:

```bash
# Установка grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Проверка File Service
grpcurl -plaintext localhost:5200 list

# Проверка Sharing Service
grpcurl -plaintext localhost:5300 list
```

---

## Решение проблем

### Проблема: Порт уже занят

```bash
# Linux/macOS - найти процесс использующий порт
lsof -i :8080
kill -9 <PID>

# Windows
netstat -ano | findstr :8080
taskkill /PID <PID> /F
```

### Проблема: Gateway не может подключиться к сервисам

1. Убедитесь что все сервисы запущены
2. Проверьте логи сервисов
3. Убедитесь что используются правильные порты
4. Проверьте firewall настройки

### Проблема: Ошибки при сборке Docker образов

```bash
# Очистка Docker кэша
docker system prune -a

# Пересборка образов
docker-compose build --no-cache
```

---

## Дополнительная информация

- **Техническое решение**: см. `docs/tr.md`
- **API документация**: см. `docs/api.md` (если есть)
- **Архитектура**: см. диаграммы в `docs/tr.md`

---

## Контакты

- Лосев Иван — тимлид, разработчик
- Киселева Софья Владимировна — разработчик

Проект: Учебный проект по курсу «Основы распределённых вычислений»