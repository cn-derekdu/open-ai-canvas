#!/usr/bin/env bash
#
# 迁移编号体检
#
# 检查 backend/internal/database/migrations.go 中 schemaMigrations 的版本号是否：
#   1) 存在重复（git 自动合并的典型产物：两处新增不相邻时不会报冲突）
#   2) 存在断号（人为顺延时跳号）
#   3) CurrentSchemaVersion 与最大迁移编号一致（漏改会导致迁移根本不执行，
#      但服务照常启动、接口照常 200，表现为"功能莫名缺失"）
#
# 用法：
#   bash scripts/check-migrations.sh [migrations.go 路径]
#
# 退出码：0 = 通过；1 = 发现问题
#
# 建议在合并上游后、go build 之前执行。

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIG_FILE="${1:-$REPO_ROOT/backend/internal/database/migrations.go}"

[ -f "$MIG_FILE" ] || {
  echo "找不到迁移文件: $MIG_FILE" >&2
  exit 1
}

echo "迁移编号体检"
echo "文件: ${MIG_FILE#"$REPO_ROOT"/}"
echo

versions="$(awk '/^var schemaMigrations/,/^}/' "$MIG_FILE" \
  | grep -oE '\{version: [0-9]+' \
  | grep -oE '[0-9]+' \
  | sort -n)"

[ -n "$versions" ] || {
  echo "未能解析出版本号，请确认 schemaMigrations 变量结构是否变化" >&2
  exit 1
}

total="$(printf '%s\n' "$versions" | wc -l | tr -d ' ')"
first="$(printf '%s\n' "$versions" | head -1)"
last="$(printf '%s\n' "$versions" | tail -1)"
declared="$(grep -oE 'CurrentSchemaVersion int64 = [0-9]+' "$MIG_FILE" | grep -oE '[0-9]+$' | head -1)"

echo "条目数: $total    范围: $first ~ $last    CurrentSchemaVersion: ${declared:-未找到}"
echo

problems=0

dups="$(printf '%s\n' "$versions" | uniq -d)"
if [ -n "$dups" ]; then
  echo "✗ 存在重复的版本号:"
  printf '%s\n' "$dups" | sed 's/^/    /'
  problems=$((problems + 1))
fi

gaps="$(printf '%s\n' "$versions" | awk 'NR>1 && $1!=prev+1 && $1!=prev {print prev" -> "$1} {prev=$1}')"
if [ -n "$gaps" ]; then
  echo "✗ 版本号不连续:"
  printf '%s\n' "$gaps" | sed 's/^/    /'
  problems=$((problems + 1))
fi

if [ -z "$declared" ]; then
  echo "✗ 未找到 CurrentSchemaVersion 常量"
  problems=$((problems + 1))
elif [ "$declared" != "$last" ]; then
  echo "✗ CurrentSchemaVersion($declared) 与最大迁移编号($last) 不一致"
  problems=$((problems + 1))
fi

echo
if [ "$problems" -eq 0 ]; then
  echo "体检通过 ✓"
  exit 0
fi

echo "体检未通过 ✗（发现 $problems 项问题）"
exit 1
