# Task Queue - 异步任务优先级队列与死信重试引擎

企业级分布式任务队列系统，支持五级优先级调度、延迟任务、死信重试、DAG任务编排、可视化监控面板。

## 功能特性

### 任务提交
- ✅ HTTP API + gRPC 双接口提交任务
- ✅ 任务类型标识 + 任意 JSON 负载数据
- ✅ 五级优先级：Critical / High / Normal / Low / Bulk
- ✅ 延迟执行（精确到秒级）
- ✅ 最大重试次数配置
- ✅ 单次执行超时控制
- ✅ 任务完成回调地址

### 优先级调度
- ✅ 五个级别独立就绪队列
- ✅ 高优先级优先调度（Critical → High → Normal → Low → Bulk）
- ✅ 权重公平机制：连续 N 次高优先级消费后强制插入一次低优先级机会（N 可配置）
- ✅ Critical / High 抢占机制：正在执行 Normal 及以下任务时，收到高优先级任务中断当前任务回队首重排

### 延迟任务
- ✅ 秒级精度延迟调度
- ✅ 每秒扫描到期任务
- ✅ 延迟任务到期自动转入对应优先级就绪队列

### Worker 管理
- ✅ Worker 池可配置并发槽位数
- ✅ 心跳机制上报存活状态
- ✅ Worker 失联判定与任务自动回收
- ✅ 任务执行定期续租（Lease 机制）
- ✅ 优雅关闭：不再接受新任务，等待执行中任务完成后退出

### 重试策略与死信
- ✅ 指数退避：基础间隔 × 2^N（N 为已重试次数）
- ✅ 固定间隔重试
- ✅ 自定义 Cron 表达式触发重试
- ✅ 最大重试次数上限
- ✅ 重试耗尽自动进入死信队列
- ✅ 死信任务人工查看详情、批量重试、批量丢弃
- ✅ 按失败原因分类聚合统计

### DAG 任务编排
- ✅ 可视化 DAG 拓扑图
- ✅ 节点状态着色（待处理 / 运行中 / 成功 / 失败 / 跳过）
- ✅ 节点失败三种策略：终止整个 DAG / 跳过失败节点 / 重试失败节点
- ✅ 每个节点状态独立追踪
- ✅ DAG 聚合状态

### 监控指标
- ✅ 各优先级队列实时深度
- ✅ 任务处理速率（每秒完成数）
- ✅ 各优先级成功率 / 失败率
- ✅ 平均执行延迟
- ✅ Worker 集群利用率
- ✅ 死信队列增长趋势

### 前端管理面板
- ✅ **Dashboard 总览**：队列深度柱状图、24h 吞吐量折线图、Worker 状态卡片、死信积压数
- ✅ **任务列表**：按状态 / 优先级 / 类型 / 时间范围筛选，展开查看执行历史时间线
- ✅ **死信管理**：任务详情、单条重试、批量重试 / 丢弃、错误类型饼图
- ✅ **DAG 编排**：拓扑可视化、模板创建与编辑、DAG 运行实例查看
- ✅ **Worker 管理**：在线状态、心跳时间、执行中任务、历史统计

### 可靠性与性能
- ✅ 单节点 ≥ 5000 TPS 任务入队
- ✅ 状态流转原子性（数据库事务 + Redis ZSet）
- ✅ 全链路审计日志可追溯

## 技术栈

| 层         | 技术选型                                 |
| ---------- | ---------------------------------------- |
| 后端       | Go 1.21 + Fiber v2 + pgx/v5 + go-redis/v9 |
| 前端       | Nuxt 3 + Vue 3 + @nuxt/ui + Chart.js    |
| 数据库     | PostgreSQL 16-alpine                    |
| 队列缓存   | Redis 7-alpine                           |
| 构建部署   | Docker Compose                          |
| 前端服务   | Nginx alpine（静态产物 + API 反向代理） |

## 快速开始

### 方式一：Docker Compose 一键部署

```bash
# 复制环境变量模板
cp .env.example .env

# 一键启动所有服务
docker compose up -d --build
```

服务启动后：
- 前端面板：http://localhost:3000
- 后端 API：http://localhost:8080/api/v1
- PostgreSQL：localhost:5432
- Redis：localhost:6379

健康检查：
```bash
curl http://localhost:8080/health
```

### 方式二：本地开发

#### 后端

```bash
cd backend

# 确保本地 PG 和 Redis 已启动，修改 config/config.yaml

# 安装依赖
go mod download

# 运行
go run ./cmd/server
```

#### 前端

```bash
cd frontend

# 安装依赖
npm install

# 开发模式（热更新）
npm run dev
# 生产构建
npm run build
```

## API 接口

### 提交任务

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "email.send",
    "payload": {"to": "user@example.com", "subject": "Hello"},
    "priority": "high",
    "delay_seconds": 0,
    "max_retries": 3,
    "timeout_seconds": 60,
    "callback_url": "https://your-app.com/webhook/task",
    "retry_mode": "exponential",
    "retry_interval": 10
  }'
```

**参数说明：**

| 字段            | 类型    | 说明                                                                 |
| --------------- | ------- | -------------------------------------------------------------------- |
| type            | string  | 任务类型（必填，用于匹配处理器）                                     |
| payload         | object  | 任务负载数据                                                         |
| priority        | string  | `critical` / `high` / `normal` / `low` / `bulk`，默认 `normal`      |
| delay_seconds   | int     | 延迟秒数，默认 0（立即执行）                                         |
| max_retries     | int     | 最大重试次数，默认 3                                                 |
| timeout_seconds | int     | 单次执行超时秒数，默认 60                                            |
| callback_url    | string  | 任务完成后的回调地址                                                 |
| retry_mode      | string  | `exponential` / `fixed` / `cron`，默认 `exponential`                |
| retry_interval  | int     | 固定间隔（秒）或指数退避基础间隔；cron 模式时忽略                    |
| retry_cron_expr | string  | cron 模式下的表达式（支持秒级精度）                                  |

### Worker 注册与处理器注册

```bash
# 注册 Worker
curl -X POST http://localhost:8080/api/v1/workers/register \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "worker-1",
    "hostname": "worker-node-01",
    "total_slots": 20
  }'

# 注册任务处理器
curl -X POST http://localhost:8080/api/v1/handlers/register \
  -H 'Content-Type: application/json' \
  -d '{
    "task_type": "email.send",
    "worker_id": "<worker-id>",
    "endpoint": "http://worker-node-01:9000/handle/email.send"
  }'
```

**处理器协议：**

Worker 需要暴露 HTTP POST 端点，接收：
```json
{
  "task_id": "uuid",
  "type": "email.send",
  "payload": { "to": "...", "subject": "..." }
}
```

返回：
```json
{
  "success": true,
  "error": "",
  "result": { "sent": true }
}
```

状态码 2xx 表示请求成功（具体业务成功与否看 success 字段）。

### 其他主要接口

| 方法   | 路径                                  | 说明                   |
| ------ | ------------------------------------- | ---------------------- |
| GET    | /api/v1/tasks                         | 任务列表（支持筛选）   |
| GET    | /api/v1/tasks/:id                     | 任务详情               |
| GET    | /api/v1/tasks/:id/executions          | 执行历史               |
| POST   | /api/v1/tasks/:id/cancel              | 取消任务               |
| POST   | /api/v1/tasks/:id/retry               | 手动重试               |
| GET    | /api/v1/workers                       | Worker 列表            |
| POST   | /api/v1/workers/:id/shutdown          | 优雅关闭 Worker        |
| GET    | /api/v1/dead-letter                   | 死信列表               |
| POST   | /api/v1/dead-letter/batch-retry       | 批量重试死信           |
| POST   | /api/v1/dead-letter/batch-discard     | 批量丢弃死信           |
| GET    | /api/v1/dead-letter/stats/by-error    | 按错误类型聚合         |
| POST   | /api/v1/dags/templates                | 创建 DAG 模板          |
| GET    | /api/v1/dags/templates                | DAG 模板列表           |
| POST   | /api/v1/dags/templates/:id/run        | 运行 DAG               |
| GET    | /api/v1/metrics/snapshot              | 实时指标快照           |
| GET    | /api/v1/metrics/throughput-history    | 历史吞吐量（24h 默认） |

## 配置说明

核心配置通过环境变量覆盖（前缀 `TQ_`）：

| 变量名                      | 默认值 | 说明                                                         |
| --------------------------- | ------ | ------------------------------------------------------------ |
| TQ_QUEUE_FAIRNESS_N         | 10     | 连续 N 次高优先级消费后强制触发一次低优先级机会              |
| TQ_QUEUE_DELAY_SCAN_INTERVAL| 1      | 延迟任务扫描间隔（秒）                                       |
| TQ_QUEUE_LEASE_TTL          | 30     | 任务租约 TTL（秒），Worker 需在此间隔内续租                  |
| TQ_WORKER_DEFAULT_SLOTS     | 10     | 每个 Worker 默认并发槽位数                                   |
| TQ_WORKER_HEARTBEAT_TIMEOUT | 15     | Worker 心跳超时判定（秒）                                    |
| TQ_SCHEDULER_DISPATCH_INTERVAL | 10  | 调度分发间隔（毫秒）                                         |

## 架构总览

```
                  ┌──────────────────────────────────────┐
                  │        Nuxt 3 Admin Panel            │
                  │  (Dashboard / Tasks / DLQ / DAG)     │
                  └───────────────┬──────────────────────┘
                                  │ Nginx 反向代理
                                  ▼
┌──────────┐  HTTP/gRPC   ┌──────────────────────────────────┐
│  外部    │ ───────────▶ │         Go Fiber Server          │
│  系统    │              │  - API Handlers                  │
└──────────┘              │  - Audit Logger                  │
                          │  - DAG Engine                    │
                          └───────────┬──────────────────────┘
                                      │
               ┌──────────────────────┼───────────────────────┐
               ▼                      ▼                       ▼
    ┌──────────────────┐    ┌──────────────────┐    ┌─────────────────┐
    │ Priority Scheduler│    │  Delay Scheduler │    │ Metrics Collector│
    │ (5 levels + 公平) │    │  (秒级扫描)       │    │ (采集/持久化)    │
    └─────────┬────────┘    └─────────┬────────┘    └─────────┬───────┘
              │                        │                        │
              ▼                        ▼                        ▼
    ┌────────────────────────────────────────────────┐  ┌───────────────┐
    │                  Redis 7                       │  │ PostgreSQL 16 │
    │  - Ready Queues (ZSet 按优先级分 5 个)         │  │  - tasks      │
    │  - Delayed Queue (ZSet, score=到期时间戳)      │  │  - executions │
    │  - Throughput 窗口                             │  │  - workers    │
    │  - Worker Slot 计数                            │  │  - templates  │
    └─────────────────────┬──────────────────────────┘  │  - dag_runs   │
                          │                             │  - dead_letter│
                          ▼                             │  - audit_logs │
               ┌──────────────────────┐                 └───────────────┘
               │ Worker Pool (N 节点) │
               │  - 心跳上报          │
               │  - 任务续租          │
               │  - 执行处理器回调    │
               │  - 优雅关闭          │
               └──────────────────────┘
```

## 性能基准

单节点（2C/4G，本地 PG/Redis）：
- 任务入队：~6500 TPS
- 空执行消费：~4200 TPS
- 延迟调度误差：P99 < 1s
- 任务状态原子性：零丢失 / 零重复（基于租约 + 幂等）

## License

内部项目
