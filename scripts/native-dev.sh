#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
use_real_home=false
dev_home_arg=
expect_dev_home=false
for arg in "$@"; do
 if "$expect_dev_home"; then dev_home_arg="$arg"; expect_dev_home=false; continue; fi
 case "$arg" in
  --use-real-home|--use-real-home=true) use_real_home=true ;;
  --dev-home) expect_dev_home=true ;;
  --dev-home=*) dev_home_arg="${arg#--dev-home=}" ;;
 esac
done
if "$expect_dev_home"; then echo '--dev-home requires a path' >&2; exit 1; fi
if [ -n "$dev_home_arg" ]; then
 if [ -n "${XIAOBAI_DEV_HOME:-}" ] && [ "$XIAOBAI_DEV_HOME" != "$dev_home_arg" ]; then
  echo '--dev-home conflicts with XIAOBAI_DEV_HOME' >&2; exit 1
 fi
 XIAOBAI_DEV_HOME="$dev_home_arg"
fi
if "$use_real_home"; then
 if [ -n "${XIAOBAI_DEV_HOME:-}" ]; then
  echo '--use-real-home conflicts with XIAOBAI_DEV_HOME' >&2; exit 1
 fi
 for arg in "$@"; do
  case "$arg" in --dev-home|--dev-home=*) echo '--use-real-home conflicts with --dev-home' >&2; exit 1;; esac
 done
fi
if [ -n "${XIAOBAI_DEV_HOME:-}" ]; then
 export XIAOBAI_DEV_HOME
 case "$XIAOBAI_DEV_HOME" in /*) ;; *) echo 'XIAOBAI_DEV_HOME must be absolute' >&2; exit 1;; esac
 if [ "$XIAOBAI_DEV_HOME" = "$HOME" ] || [ "$XIAOBAI_DEV_HOME" = / ]; then
  echo 'Refusing to use the real home or root for isolated development' >&2
  exit 1
 fi
 printf 'Isolated native development home: %s\n' "$XIAOBAI_DEV_HOME"
else
 printf 'Native development uses existing user data; desktop integrations and updates remain disabled\n'
fi
./scripts/native-go.sh build -ldflags "-X github.com/licoy/xiaobai-switch/internal/buildinfo.Version=1.0.0-dev" -o .build/native-dev-XiaoBaiSwitch .
exec .build/native-dev-XiaoBaiSwitch "$@"
