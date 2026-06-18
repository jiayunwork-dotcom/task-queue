.PHONY: help build up down clean logs backend-logs frontend-logs ps test-backend seed

.DEFAULT_GOAL := help

help: ## 显示帮助信息
	@awk 'BEGIN {FS = ":.*##"; printf "\n用法:\n  make \033[36m<目标>\033[0m\n\n目标:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## 构建所有容器
	docker compose build

up: ## 启动所有服务
	docker compose up -d

down: ## 停止并删除所有容器
	docker compose down

down-v: ## 停止删除容器和数据卷（清库）
	docker compose down -v

logs: ## 查看所有服务日志
	docker compose logs -f --tail=200

backend-logs: ## 只看后端日志
	docker compose logs -f --tail=200 backend

frontend-logs: ## 只看前端日志
	docker compose logs -f --tail=200 frontend

ps: ## 查看服务状态
	docker compose ps

restart: ## 重启所有服务
	docker compose restart

restart-backend: ## 只重启后端
	docker compose restart backend

seed: up ## 提交测试任务（健康检查后）
	@echo "等待服务就绪..."
	@sleep 3
	@curl -s -X POST http://localhost:8080/api/v1/tasks \
		-H 'Content-Type: application/json' \
		-d '{"type":"__echo__","payload":{"hello":"world"},"priority":"high","max_retries":3}' \
		| python3 -m json.tool || echo "提交失败，请先运行 make up"

benchmark: up ## 压测：快速提交1000个任务
	@echo "提交1000个测试任务..."
	@for i in $(shell seq 1 1000); do \
		curl -s -X POST http://localhost:8080/api/v1/tasks \
			-H 'Content-Type: application/json' \
			-d "{\"type\":\"__echo__\",\"payload\":{\"id\":$$i},\"priority\":\"normal\"}" > /dev/null; \
	done
	@echo "完成！查看 Dashboard http://localhost:3000"

health: ## 健康检查
	@echo "后端:  "
	@curl -s http://localhost:8080/health | python3 -m json.tool || echo "未就绪"
	@echo "前端:  "
	@curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:3000 || echo "未就绪"
