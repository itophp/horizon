#!/usr/bin/env bash
# 两套独立栈并行跑完浏览器集合：默认栈负责管理/基础/MCP，
# governance 配置栈（5183/8101）负责治理/通知/设置排期。
set -u
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root" || exit 1

logs="$root/apps/web/.local"
mkdir -p "$logs" || exit 1

default_specs=(
  e2e/administration.spec.ts
  e2e/foundation.spec.ts
  e2e/mcp.spec.ts
  e2e/webmcp.spec.ts
)

pnpm test:e2e "${default_specs[@]}" >"$logs/e2e-default.log" 2>&1 &
default_pid=$!
pnpm test:e2e --config=playwright.governance.config.ts \
  >"$logs/e2e-governance.log" 2>&1 &
governance_pid=$!

default_status=0
governance_status=0
wait "$default_pid" || default_status=$?
wait "$governance_pid" || governance_status=$?

report() {
  local label=$1 status=$2 log=$3
  if [ "$status" -eq 0 ]; then
    echo "$label 通过"
    return
  fi
  echo "$label 失败（退出码 $status），日志 $log"
  tail -n 40 "$log"
}

report "默认栈" "$default_status" "$logs/e2e-default.log"
report "governance 栈" "$governance_status" "$logs/e2e-governance.log"

[ "$default_status" -eq 0 ] && [ "$governance_status" -eq 0 ]
