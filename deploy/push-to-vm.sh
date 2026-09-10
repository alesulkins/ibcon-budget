#!/usr/bin/env bash
# запускается на ноутике: копирует проект на сервер и разворачивает.
#
#   deploy/push-to-vm.sh angela@192.168.24.6 ~/.ssh/ibcon_vm
set -euo pipefail

target=${1:?укажите пользователь@адрес}
key=${2:-~/.ssh/ibcon_vm}
dir=${3:-ibcon-budget}

cd "$(dirname "$0")/.."

# Везём исходники: сборку, зависимости и рабочие материалы сервер не ждёт.
rsync -az --delete \
  --exclude '.git' \
  --exclude 'node_modules' \
  --exclude 'frontend/dist' \
  --exclude '*.xlsm' \
  --exclude '.env' \
  --exclude 'audit' --exclude 'docs' --exclude 'mapping' --exclude '.claude' \
  -e "ssh -i $key" ./ "$target:$dir/"

ssh -i "$key" "$target" "cd $dir && bash deploy/vm-setup.sh"
