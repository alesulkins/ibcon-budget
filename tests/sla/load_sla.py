#!/usr/bin/env python3
"""Нагрузочная проверка нормативов времени отклика (SLA Минцифры).

Меряет то же, что видит пользователь, но по API — без браузера:
десять человек работают одновременно, каждый проходит цепочку
«реестр → карточка проекта → версия бюджета → расчёт → выгрузка».

Нормативы (см. THRESHOLDS): карточка проекта — 5 с, открытие версии
бюджета — 10 с, расчёт — 2 мин, выгрузка XLSX — 1 мин.

Только стандартная библиотека: скрипт должен запускаться на машине
проверяющего без установки зависимостей.

    python3 tests/sla/load_sla.py --users 10
    python3 tests/sla/load_sla.py --url http://10.0.0.5:8080 --version 23

Код возврата: 0 — все нормативы выдержаны, 1 — есть превышение.
"""

import argparse
import json
import statistics
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor

# Норматив на операцию, секунды.
THRESHOLDS = {
    'Реестр проектов': 5.0,
    'Карточка проекта': 5.0,
    'Открытие версии бюджета': 10.0,
    'Расчёт бюджета': 120.0,
    'Выгрузка XLSX': 60.0,
}

# Замер считается пройденным по 95-му перцентилю: одиночный выброс на
# чужой нагрузке норматив не рушит, а систематическая просадка — рушит.
PERCENTILE = 95


# Запросы идут мимо системного прокси: замер должен мерить сервер, а не
# прокси-сервер рабочей станции. Без этого urllib на macOS подхватывает
# настройки системы и отдаёт чужой 503 вместо ответа приложения.
_opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def call(url, path, token=None, method='GET', body=None, timeout=180):
    """Один запрос. Возвращает (секунды, тело ответа в байтах)."""
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url + path, data=data, method=method)
    req.add_header('Content-Type', 'application/json')
    if token:
        req.add_header('Authorization', 'Bearer ' + token)
    start = time.perf_counter()
    with _opener.open(req, timeout=timeout) as resp:
        payload = resp.read()
    return time.perf_counter() - start, payload


def login(url, email, password):
    _, payload = call(url, '/api/v1/auth/login', method='POST',
                      body={'email': email, 'password': password,
                            'remember_me': False})
    return json.loads(payload)['token']


def discover(url, token, project=None, version=None):
    """Находит проект с версией бюджета, если их не задали руками."""
    if project and version:
        return project, version
    _, payload = call(url, '/api/v1/projects?limit=100', token)
    items = json.loads(payload).get('items', [])
    for item in items:
        pid = item['id']
        try:
            _, vs = call(url, f'/api/v1/projects/{pid}/budgets/versions', token)
        except urllib.error.HTTPError:
            continue  # нет доступа к проекту — не наш случай
        versions = json.loads(vs) or []
        if versions:
            return pid, versions[0]['id']
    sys.exit('Не найден проект с версией бюджета — задайте --project и --version')


def scenario(url, token, pid, vid):
    """Цепочка одного пользователя. Возвращает {операция: секунды}."""
    out = {}
    out['Реестр проектов'], _ = call(url, '/api/v1/projects?limit=20', token)
    out['Карточка проекта'], _ = call(url, f'/api/v1/projects/{pid}', token)

    # Открытие версии — это то, что грузит экран мастера: сама версия,
    # её вводные и список соседних версий. Норматив на весь экран,
    # поэтому и меряем всё разом, а не один запрос.
    start = time.perf_counter()
    call(url, f'/api/v1/budget-versions/{vid}', token)
    call(url, f'/api/v1/budget-versions/{vid}/inputs', token)
    call(url, f'/api/v1/projects/{pid}/budgets/versions', token)
    out['Открытие версии бюджета'] = time.perf_counter() - start

    out['Расчёт бюджета'], _ = call(url, f'/api/v1/budget-versions/{vid}/calculate', token)
    out['Выгрузка XLSX'], book = call(url, f'/api/v1/budget-versions/{vid}/export', token)
    if not book.startswith(b'PK'):
        raise RuntimeError('Выгрузка вернула не xlsx (нет сигнатуры ZIP)')
    return out


def percentile(values, pct):
    if len(values) == 1:
        return values[0]
    ordered = sorted(values)
    # Линейная интерполяция — как в statistics.quantiles, но без
    # ограничения на длину выборки.
    pos = (len(ordered) - 1) * pct / 100
    low = int(pos)
    high = min(low + 1, len(ordered) - 1)
    return ordered[low] + (ordered[high] - ordered[low]) * (pos - low)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--url', default='http://localhost:8080')
    ap.add_argument('--users', type=int, default=10)
    ap.add_argument('--email', default='admin@ibcon.ru')
    ap.add_argument('--password', default='IBcon2024Admin!')
    ap.add_argument('--project', type=int)
    ap.add_argument('--version', type=int)
    args = ap.parse_args()

    token = login(args.url, args.email, args.password)
    pid, vid = discover(args.url, token, args.project, args.version)
    print(f'Проект {pid}, версия бюджета {vid}, одновременных пользователей: {args.users}\n')

    # Все пользователи стартуют разом: нагрузка должна быть
    # одновременной, иначе это не проверка одновременной работы.
    with ThreadPoolExecutor(max_workers=args.users) as pool:
        futures = [pool.submit(scenario, args.url, token, pid, vid)
                   for _ in range(args.users)]
        results, failures = [], []
        for f in futures:
            try:
                results.append(f.result())
            except Exception as exc:  # noqa: BLE001 — печатаем и продолжаем
                failures.append(repr(exc))

    if failures:
        print(f'Отказов: {len(failures)}')
        for f in failures[:5]:
            print('  ', f)
        print()

    print(f'{"Операция":<26}{"p" + str(PERCENTILE):>9}{"макс":>9}{"норматив":>11}  итог')
    ok = not failures
    for name, limit in THRESHOLDS.items():
        values = [r[name] for r in results if name in r]
        if not values:
            continue
        p, worst = percentile(values, PERCENTILE), max(values)
        passed = p <= limit
        ok = ok and passed
        print(f'{name:<26}{p:>8.2f}с{worst:>8.2f}с{limit:>10.0f}с  '
              f'{"ВЫДЕРЖАН" if passed else "ПРЕВЫШЕН"}')

    if results:
        total = statistics.mean(sum(r.values()) for r in results)
        print(f'\nСредняя длительность полного сценария: {total:.2f} с')
    return 0 if ok else 1


if __name__ == '__main__':
    sys.exit(main())
