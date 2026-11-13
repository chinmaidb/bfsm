#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$(cd -- "$SCRIPT_DIR/../frontend" && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"
ENV_FILE="$SCRIPT_DIR/.env"
EXAMPLE_ENV_FILE="$SCRIPT_DIR/.env.example"
PRIVATE_KEY="$SCRIPT_DIR/secrets/jwt-private.pem"
PUBLIC_KEY="$SCRIPT_DIR/secrets/jwt-public.pem"
PM2_CONFIG="$SCRIPT_DIR/nginx/ecosystem.config.js"
APP_NAME="benefit-scheme-api"

build_frontend() {
  command -v pnpm >/dev/null 2>&1 || {
    printf 'pnpm is required to build the frontend.\n' >&2
    return 1
  }
  (
    cd "$FRONTEND_DIR"
    pnpm install --frozen-lockfile
    pnpm build
  )

  mkdir -p "$BUILD_DIR"
  rm -rf -- "$BUILD_DIR/bfsm-web"
  mkdir -p "$BUILD_DIR/bfsm-web"
  cp -a "$FRONTEND_DIR/dist/." "$BUILD_DIR/bfsm-web/"
}

build_backend() {
  mkdir -p "$BUILD_DIR"
  (
    cd "$SCRIPT_DIR"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go build -trimpath -ldflags="-s -w" -o "$BUILD_DIR/bfsm-api" ./cmd/api
  )
}

create_env() {
  if [[ -e "$ENV_FILE" ]]; then
    printf 'Keeping existing environment file: %s\n' "$ENV_FILE"
    return
  fi
  if [[ ! -f "$EXAMPLE_ENV_FILE" ]]; then
    printf 'Environment template not found: %s\n' "$EXAMPLE_ENV_FILE" >&2
    return 1
  fi
  if ! command -v openssl >/dev/null 2>&1; then
    printf 'openssl is required to generate the initial super-admin password.\n' >&2
    return 1
  fi

  umask 077
  local password
  password="$(openssl rand -hex 32)"
  cp "$EXAMPLE_ENV_FILE" "$ENV_FILE"
  if grep -q '^SUPER_ADMIN_PASSWORD=' "$ENV_FILE"; then
    sed -i "s|^SUPER_ADMIN_PASSWORD=.*|SUPER_ADMIN_PASSWORD=$password|" "$ENV_FILE"
  else
    printf '\nSUPER_ADMIN_PASSWORD=%s\n' "$password" >> "$ENV_FILE"
  fi
  sed -i 's/^APP_ENV=.*/APP_ENV=production/' "$ENV_FILE"
  if grep -q '^WEB_DIR=' "$ENV_FILE"; then
    sed -i "s|^WEB_DIR=.*|WEB_DIR=$BUILD_DIR/bfsm-web|" "$ENV_FILE"
  else
    printf '\nWEB_DIR=%s/bfsm-web\n' "$BUILD_DIR" >> "$ENV_FILE"
  fi
  chmod 600 "$ENV_FILE"
  printf 'Created %s. Review DATABASE_URL and other deployment settings before proceeding.\n' "$ENV_FILE"
}

generate_keys() {
  if [[ -e "$PRIVATE_KEY" || -e "$PUBLIC_KEY" ]]; then
    if [[ -f "$PRIVATE_KEY" && -f "$PUBLIC_KEY" ]]; then
      printf 'Keeping existing JWT keypair.\n'
      return
    fi
    printf 'Only one JWT key exists; refusing to overwrite or replace it.\n' >&2
    return 1
  fi
  mkdir -p "$SCRIPT_DIR/secrets"
  chmod 700 "$SCRIPT_DIR/secrets"
  (
    cd "$SCRIPT_DIR"
    go run ./cmd/keygen "$PRIVATE_KEY" "$PUBLIC_KEY"
  )
  printf 'Generated JWT keypair in %s/secrets.\n' "$SCRIPT_DIR"
}

run_migrations() {
  if [[ ! -f "$ENV_FILE" ]]; then
    printf 'Environment file not found. Run "%s env" and configure DATABASE_URL first.\n' "$0" >&2
    return 1
  fi
  (
    cd "$SCRIPT_DIR"
    go run ./cmd/migrate
  )
}

require_pm2() {
  if ! command -v pm2 >/dev/null 2>&1; then
    printf 'PM2 is required. Install it with: npm install --global pm2\n' >&2
    return 1
  fi
}

start_app() {
  require_pm2
  if [[ ! -f "$ENV_FILE" ]]; then
    printf 'Environment file not found. Run "%s env" and configure it first.\n' "$0" >&2
    return 1
  fi
  if [[ ! -x "$BUILD_DIR/bfsm-api" ]]; then
    printf 'Go executable not found. Run "%s build" first.\n' "$0" >&2
    return 1
  fi
  if [[ ! -f "$BUILD_DIR/bfsm-web/index.html" ]]; then
    printf 'Frontend build not found. Run "%s frontend" first.\n' "$0" >&2
    return 1
  fi
  if [[ ! -f "$PRIVATE_KEY" ]]; then
    printf 'JWT private key not found. Run "%s keygen" first.\n' "$0" >&2
    return 1
  fi

  pm2 startOrReload "$PM2_CONFIG" --env production
}

restart_app() {
  build_frontend
  build_backend
  run_migrations
  require_pm2
  start_app
}

action="${1:-all}"
case "$action" in
  all)
    build_frontend
    build_backend
    create_env
    generate_keys
    run_migrations
    start_app
    ;;
  build)
    build_frontend
    build_backend
    ;;
  frontend)
    build_frontend
    ;;
  backend)
    build_backend
    ;;
  env)
    create_env
    ;;
  keygen)
    generate_keys
    ;;
  migrate)
    run_migrations
    ;;
  start|up)
    start_app
    ;;
  restart)
    restart_app
    ;;
  stop)
    require_pm2
    pm2 stop "$APP_NAME"
    ;;
  delete|remove)
    require_pm2
    pm2 delete "$APP_NAME"
    ;;
  status)
    require_pm2
    pm2 status "$APP_NAME"
    ;;
  logs)
    require_pm2
    shift || true
    pm2 logs "$APP_NAME" "$@"
    ;;
  *)
    printf 'Usage: %s [all|build|frontend|backend|env|keygen|migrate|start|up|restart|stop|delete|status|logs]\n' "$0" >&2
    exit 2
    ;;
esac
