# MU 框架 · 插件开发指南

## 一、插件生命周期

```
外部代码 ──Install──▶ Loaded ──Start──▶ Running
                        ▲                 │
                        └─────Stop────────┘
```

## 二、实现 Plugin 接口

```go
type Plugin interface {
    Meta() Meta                        // 元信息
    Init(ctx context.Context) error    // 一次性初始化
    Start() error                      // 启动
    Stop() error                       // 停止
    Health() HealthStatus              // 健康检查
}
```

## 三、示例

参考：`backend/internal/agent/plugin/examples/hello/plugin.go`

- 每秒自增计数（后台 goroutine）
- `Count() int64` / `Sayhi(name string) string`
- `Stop()` 优雅关闭

## 四、依赖管理

```go
func (p *MyPlugin) Meta() plugin.Meta {
    return plugin.Meta{
        ID:           "my-plugin",
        Version:      "1.0.0",
        Dependencies: []string{"hello"},
        MinFramework: "1.0.0",
    }
}
```

## 五、在线热插拔

```go
mgr := eng.GetPluginManager()
_ = mgr.Install(ctx, hello.New())
_ = mgr.Start("hello")

// 运行时热卸载
_ = mgr.Stop("hello")
_ = mgr.Uninstall("hello")
```

## 六、远程 .so 插件（生产）

1. 插件编译为 `.so`（`go build -buildmode=plugin`）
2. 上传到开发商后台"插件管理"
3. 框架从存储中台拉取 `.so` → `plugin.Open()` → 调用导出符号 `New()`
4. 调用 `mgr.Install(ctx, p)` 注入

## 七、最佳实践

1. **幂等初始化**：`Init` 应可多次调用不产生副作用
2. **优雅停止**：`Stop` 必须清理所有 goroutine/连接
3. **失败自愈**：`Health` 返回不健康时，框架会触发自动重启
4. **权限绑定**：通过权限引擎注册插件权限编码（`plugin.xxx:*`）
5. **租户限制**：单租户 / 白名单 / 全部
