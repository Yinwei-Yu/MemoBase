# MemoBase 监控系统

## 组件

| 组件 | 镜像 | 端口 | 用途 |
|---|---|---|---|
| Prometheus | prom/prometheus | 9090 | 指标采集与存储 |
| Grafana | grafana/grafana | 3000 | 可视化 Dashboard |
| cAdvisor | gcr.io/cadvisor/cadvisor | 8081 | 容器资源监控 |
| Node Exporter | prom/node-exporter | 9100 | 主机指标监控 |

## 快速访问

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)
- cAdvisor: http://localhost:8081

## 采集目标

Prometheus 配置了以下采集目标：

1. **memobase-backend** - 应用指标
   - HTTP 请求总数、延迟、在途请求数
   - 路径: `/metrics`

2. **cadvisor** - 容器指标
   - CPU、内存、网络、磁盘使用
   - 路径: `/metrics`

3. **node-exporter** - 主机指标
   - 系统负载、磁盘空间、网络流量
   - 路径: `/metrics`

## 预置 Dashboard

### MemoBase Overview

包含以下面板：

1. **Service Status** - 后端服务状态 (up/down)
2. **In-Flight Requests** - 当前处理中的请求数
3. **Request Rate by Status** - 按状态码的请求速率趋势
4. **Request Duration by Route** - 按路由的平均请求延迟
5. **Container Memory Usage** - 各容器内存使用趋势
6. **Container CPU Usage** - 各容器 CPU 使用趋势

## 告警规则（可选）

可在 `prometheus.yml` 中添加告警规则：

```yaml
rule_files:
  - 'alert_rules.yml'

alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']
```

示例告警规则 (`alert_rules.yml`)：

```yaml
groups:
  - name: memobase
    rules:
      - alert: ServiceDown
        expr: memobase_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "MemoBase backend is down"

      - alert: HighErrorRate
        expr: rate(memobase_http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
```

## 数据持久化

- Prometheus 数据存储在 `prometheus_data` 卷
- Grafana 数据存储在 `grafana_data` 卷

## 生产环境

生产环境下所有监控端口不对外暴露，需要通过反向代理或 VPN 访问。
