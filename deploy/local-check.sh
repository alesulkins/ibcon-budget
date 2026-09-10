#!/usr/bin/env bash
# Проверка на своём компьютере: поднимает боевую конфигурацию так же, как
# она поднимется на сервере, показывает состояние служб и убирает за собой.
# Порт 8081, чтобы не мешать другим службам; данные — во временном томе.
set -euo pipefail

cd "$(dirname "$0")/.."
project=ibcon-selfcheck
env_file=$(mktemp)
ports=$(mktemp -t ports.XXXX.yml)

cat > "$env_file" <<ENV
DB_USER=ibcon
DB_PASSWORD=selfcheck_$(date +%s)
DB_NAME=ibcon_budget
JWT_SECRET=selfcheck_secret_at_least_32_characters_long
ADMIN_EMAIL=admin@ibcon.ru
ENV

cat > "$ports" <<'YML'
services:
  frontend:
    ports:
      - "8081:80"
YML

compose() {
  docker compose --env-file "$env_file" \
    -f docker-compose.prod.yml -f "$ports" -p "$project" "$@"
}

cleanup() {
  echo
  echo "Убираю проверочный стенд"
  compose down -v >/dev/null 2>&1 || true
  rm -f "$env_file" "$ports"
}
trap cleanup EXIT

echo "Собираю и поднимаю (первый раз — несколько минут)"
compose up -d --build

echo -n "Жду готовности"
for _ in $(seq 1 60); do
  if curl -fsS -o /dev/null http://localhost:8081/ 2>/dev/null; then
    echo " — готово"
    break
  fi
  echo -n "."
  sleep 2
done

echo
echo "Состояние служб:"
compose ps --format "table {{.Service}}\t{{.Status}}"

echo
echo "Проверки:"
echo "  интерфейс: $(curl -s -o /dev/null -w '%{http_code}' http://localhost:8081/) (ожидается 200)"
echo "  API: $(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:8081/api/v1/auth/login \
        -H 'Content-Type: application/json' -d '{"email":"нет@нет.ru","password":"x"}') (ожидается 401)"

tables=$(compose exec -T postgres psql -U ibcon -d ibcon_budget -t -c \
         "select count(*) from information_schema.tables where table_schema='public';" 2>/dev/null | tr -d ' ')
echo "  таблиц в базе: ${tables:-?} (миграции прошли)"

users=$(compose exec -T postgres psql -U ibcon -d ibcon_budget -t -c \
        "select count(*) from users;" 2>/dev/null | tr -d ' ')
echo "  учётных записей: ${users:-?} (создаётся первая)"

pass=$(compose logs backend 2>/dev/null | grep -o '"пароль":"[^"]*"' | tail -1 | cut -d'"' -f4)
echo "  пароль первого входа: ${pass:-не найден в журнале}"

echo
echo "Открыть в браузере: http://localhost:8081/  (вход admin@ibcon.ru)"
echo "Нажмите Enter, когда посмотрите — стенд будет удалён."
read -r _
