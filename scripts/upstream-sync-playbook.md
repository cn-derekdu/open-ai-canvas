# 上游同步与上线 Playbook（个人二开线）

> 触发词：**「更新到最新版」**、**「同步上游」**、**「更新到最新版并部署」**
> 范围：`cn-derekdu/open-ai-canvas` fork 的私有工作流。本文件不属于上游内容，禁止向上游提交或带入 PR。

## 0. 固定背景

| 项 | 值 |
| --- | --- |
| 本地仓库 | `/Users/derekdu/Documents/ai项目/open-ai-canvas` |
| `origin` | `git@github.com:cn-derekdu/open-ai-canvas.git`：私有二开线，功能提交直接落 `main` |
| `upstream` | `git@github.com:ddcat-ai/open-ai-canvas.git`：只读上游 |
| 同步方式 | rebase，保持线性历史 |
| 服务器 | `ssh ubuntu-93`（root@93.177.76.177:15949） |
| 部署目录 | `/opt/open-ai-canvas` |
| 部署形态 | `docker-compose.deploy.yml` + `docker-compose.build.yml` 源码构建，主机端口 3001 |
| 对外入口 | Caddy 反代 `routerbox.cc` / `www.routerbox.cc` → `127.0.0.1:3001`；`api.routerbox.cc` 属于 new-api 服务，禁止改动 |
| PR 分支 | 一律 `git switch -c <branch> upstream/main`，不从 fork 的 `main` 切 |

## 1. 阶段一：只读分析（此阶段不得改代码）

```bash
git fetch upstream --tags --prune
git --no-pager log --oneline HEAD..upstream/main
git --no-pager diff --stat HEAD..upstream/main
```

必须产出：

- 上游新提交按 `feat|fix|refactor|perf|docs|test|build|ci|chore|revert` 归类
- 最新 release tag 与主要变化（如果上游打了 tag）
- 与二开有交集的文件清单：`web/package.json`、`bun.lock`、`backend/go.mod`、`go.sum`、`AGENTS.md`、数据库迁移，以及二开改动过的文件

产出后停下等用户确认，再进入下一阶段。

## 2. 阶段二：备份

```bash
ssh ubuntu-93 'mkdir -p /opt/backup && docker exec open-ai-canvas-postgres-1 pg_dump -U open_ai_canvas open_ai_canvas | gzip > /opt/backup/canvas-$(date +%F-%H%M).sql.gz && ls -lh /opt/backup | tail -3'
```

同时确认：`/opt/open-ai-canvas/.env` 存在且权限 600；数据卷 `open-ai-canvas_postgres-data`、`open-ai-canvas_backend-data`、`open-ai-canvas_redis-data` 完好。迁移没有 down 脚本，备份是唯一回退手段。

## 3. 阶段三：本地 rebase

```bash
git switch main
git status --porcelain          # 必须为空，否则先处理干净
git rebase upstream/main
```

冲突处理原则：

- 公共逻辑：以上游改动为准，同时保留二开的功能意图；判断不了就停下来问
- 锁文件（`bun.lock`、`go.mod`、`go.sum`）：采用上游版本后重新安装，禁止手改
- 重复提交或空提交（说明该改动已被上游合并）：`git rebase --skip`
- 放弃本次同步：`git rebase --abort`

## 4. 阶段四：本地验证

```bash
cd web && bun run typecheck && bun run lint && bun run test:canvas
cd ../backend && go build ./...        # 改动涉及后端时
```

- 涉及画布、生成、权限、SSE 时按 `AGENTS.md` 第 8 节补最小充分验证
- 失败用例要区分"本次引入"与"既有问题"，不得跳过

## 5. 阶段五：推送

```bash
git push --force-with-lease origin main     # 禁止 git push --force
```

## 6. 阶段六：服务器更新

```bash
ssh ubuntu-93 'cd /opt/open-ai-canvas && git fetch origin main && git reset --hard origin/main && git log --oneline -1'
ssh ubuntu-93 'cd /opt/open-ai-canvas && docker compose --env-file .env -f docker-compose.deploy.yml -f docker-compose.build.yml up -d --build --remove-orphans'
```

- 只改前端时可把构建目标缩到 `web`（`... up -d --build web`）加快发布
- `reset --hard` 不影响 `.env` 与数据卷（均不在版本库内）

## 7. 阶段七：线上验证

```bash
ssh ubuntu-93 'docker ps --format "{{.Names}}: {{.Status}}" | grep open-ai-canvas'
ssh ubuntu-93 'curl -s -o /dev/null -w "%{http_code}\n" https://routerbox.cc/'
ssh ubuntu-93 'cd /opt/open-ai-canvas && docker compose --env-file .env -f docker-compose.deploy.yml -f docker-compose.build.yml logs --tail=100 backend web | grep -iE "error|panic|fatal"'
```

- UI 改动还要核验产物内容：`docker exec open-ai-canvas-web-1 sh -c "grep -rl <新增字符串> /usr/share/nginx/html/assets"`
- 健康检查和 200 响应不能替代登录、SSE、任务生成、资源访问的实测

## 8. 汇报格式

1. 上游新版本号与关键变化
2. 与二开的交集文件、冲突与解决方式
3. 本地验证命令与结果
4. 线上验证结果（容器状态、域名、产物核验）
5. 遗留问题与后续建议
6. 回滚步骤：
   - 代码：`git reset --hard <旧 commit>` 后重建镜像
   - 数据库：`gunzip -c /opt/backup/canvas-xxx.sql.gz | docker exec -i open-ai-canvas-postgres-1 psql -U open_ai_canvas open_ai_canvas`

## 9. 硬约束

- 不提交 `.env`、密钥、数据库、日志、本机配置
- 不动 `/etc/caddy/Caddyfile` 中 `api.routerbox.cc` 的配置
- 未经用户确认，不做 `reset --hard`、分支删除、远程历史改写
- 项目官方文档与个人要求冲突时，以官方文档为准，并先说明冲突点
- 同步完成后，`main` 必须与 `origin/main` 一致且工作区干净（`git status -sb` 无残留）
