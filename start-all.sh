#!/bin/bash

# Скрипт для запуска всех сервисов в отдельных терминалах

echo "🚀 Starting all services..."
echo ""

# Проверяем наличие tmux
if ! command -v tmux &> /dev/null; then
    echo "❌ tmux не установлен. Установите его: sudo apt-get install tmux"
    exit 1
fi

# Создаем новую tmux сессию
SESSION="cloud-storage"

# Убиваем существующую сессию если есть
tmux kill-session -t $SESSION 2>/dev/null

# Создаем новую сессию с первым окном для Auth
tmux new-session -d -s $SESSION -n "Auth"
tmux send-keys -t $SESSION:0 "cd $(pwd) && ./start-auth.sh" C-m

# Создаем окно для File Service
tmux new-window -t $SESSION:1 -n "File"
tmux send-keys -t $SESSION:1 "cd $(pwd) && ./start-file.sh" C-m

# Создаем окно для Sharing Service
tmux new-window -t $SESSION:2 -n "Sharing"
tmux send-keys -t $SESSION:2 "cd $(pwd) && ./start-sharing.sh" C-m

# Создаем окно для Gateway
tmux new-window -t $SESSION:3 -n "Gateway"
tmux send-keys -t $SESSION:3 "cd $(pwd) && sleep 3 && ./start-gateway.sh" C-m

# Подключаемся к сессии
echo "✅ Все сервисы запущены в tmux сессии '$SESSION'"
echo ""
echo "Для просмотра сервисов используйте:"
echo "  tmux attach -t $SESSION"
echo ""
echo "Переключение между окнами:"
echo "  Ctrl+B, затем 0-3 (номер окна)"
echo ""
echo "Отключение от сессии (сервисы продолжат работать):"
echo "  Ctrl+B, затем D"
echo ""
echo "Остановка всех сервисов:"
echo "  tmux kill-session -t $SESSION"
echo ""

tmux attach -t $SESSION