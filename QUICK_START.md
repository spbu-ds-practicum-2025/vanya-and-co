# 🚀 Быстрый старт

## Выберите способ запуска:

### 🐳 Вариант 1: Docker (Рекомендуется)

```bash
# Запуск всех сервисов
docker-compose up

# В фоновом режиме
docker-compose up -d
```

Откройте http://localhost:8080

---

### 💻 Вариант 2: Локальный запуск

#### Linux/macOS с tmux:
```bash
chmod +x start-all.sh
./start-all.sh
```

#### Linux/macOS без tmux (4 терминала):
```bash
# Терминал 1
./start-auth.sh

# Терминал 2
./start-file.sh

# Терминал 3
./start-sharing.sh

# Терминал 4 (подождите 2-3 секунды)
./start-gateway.sh
```

#### Windows:
Дважды кликните `start-all.bat` или запустите в 4 окнах командной строки:
```cmd
start-auth.bat
start-file.bat
start-sharing.bat
start-gateway.bat
```

---

## 📋 Что дальше?

1. Откройте http://localhost:8080
2. Нажмите "Регистрация"
3. Создайте аккаунт
4. Загрузите файлы!

---

## 🛑 Остановка

**Docker:**
```bash
docker-compose down
```

**Локальный запуск:**
- Linux/macOS с tmux: `tmux kill-session -t cloud-storage`
- Остальные: Ctrl+C в каждом терминале

---

## 📚 Подробная документация

См. [README_RUN.md](README_RUN.md) для полной информации.

## 🧪 Тестирование изоляции

Попробуйте остановить отдельные сервисы и посмотрите, как система продолжает работать!

Например:
- Остановите Auth → Gateway работает, но вход недоступен
- Остановите File → Авторизация работает, но файлы недоступны
- Остановите Sharing → Основные функции работают, обмен недоступен