#!/usr/bin/env bash
# запускается НА СЕРВЕРЕ из каталога проекта: готовит .env, поднимает
# контейнеры и ждёт, пока система ответит. Повторный запуск безопасен —
# существующий .env не перезаписывается, данные базы не трогаются.
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v docker >/dev/null; then
  echo "Docker не установлен" >&2
  exit 1
fi

if [ ! -f .env ]; then
  echo "Создаю .env со случайными паролями"
  cp .env.prod.example .env
  db_pass=$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)
  jwt=$(openssl rand -base64 48 | tr -d '/+=' | cut -c1-48)
  # -i.bak: BSD и GNU sed по-разному понимают -i без аргумента
  sed -i.bak "s|^DB_PASSWORD=.*|DB_PASSWORD=${db_pass}|" .env
  sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=${jwt}|" .env
  rm -f .env.bak
  chmod 600 .env
else
  echo ".env уже есть — оставляю как есть"
fi

echo "Собираю образы и поднимаю контейнеры"
docker compose -f docker-compose.prod.yml up -d --build

echo -n "Жду, пока бэкенд ответит"
for _ in $(seq 1 60); do
  if docker compose -f docker-compose.prod.yml exec -T backend \
       wget -qO- http://localhost:8080/health >/dev/null 2>&1; then
    echo " — готово"
    docker compose -f docker-compose.prod.yml ps
    ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    echo
    echo "Система доступна: http://${ip:-<адрес сервера>}/"

    # Пароль первой учётной записи печатается сервером один раз при
    # создании — достаём его из журнала, иначе войти будет нечем.
    pass=$(docker compose -f docker-compose.prod.yml logs backend 2>/dev/null \
           | grep -o '"пароль":"[^"]*"' | tail -1 | cut -d'"' -f4)
    if [ -n "$pass" ]; then
      user=$(grep '^ADMIN_EMAIL=' .env | cut -d= -f2)
      echo "Вход: ${user:-admin@ibcon.ru} / ${pass}"
      echo "Пароль сгенерирован при первом запуске — смените его после входа."
    fi
    exit 0
  fi
  echo -n "."
  sleep 2
done

echo " — не дождался. Журнал бэкенда:" >&2
docker compose -f docker-compose.prod.yml logs --tail 40 backend >&2
exit 1
