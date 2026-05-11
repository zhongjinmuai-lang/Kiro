// Package plugin v2.3 插件沙箱加固
//
// 能力：
//   - 版本兼容校验（MinFramework 与当前框架版本比较）
//   - 拓扑排序启动（按依赖图 DAG 顺序加载）
//   - 运行时资源监控（goroutine 计数 + panic 隔离）
//   - 插件级别权限绑定（与三级管控对接）
package plugin

import (
	"fmt"
	"sort"
	"strings"
)

// FrameworkVersion 当前框架版本
const FrameworkVersion = "2.3.0"

// CompareVersion 简易版本比较（a >= b 返回 true）
// 支持 major.minor.patch 格式
func CompareVersion(a, b string) bool {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		va, vb := 0, 0
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &va)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &vb)
		}
		if va > vb {
			return true
		}
		if va < vb {
			return false
		}
	}
	return true // 相等
}

// ValidateVersion 校验插件最低框架版本要求
func ValidateVersion(minFramework string) error {
	if minFramework == "" {
		return nil
	}
	if !CompareVersion(FrameworkVersion, minFramework) {
		return fmt.Errorf("插件要求框架版本 >= %s，当前版本 %s", minFramework, FrameworkVersion)
	}
	return nil
}

// TopologicalSort 对插件按依赖关系拓扑排序
// 返回启动顺序（被依赖的先启动）
// 如有循环依赖返回错误
func TopologicalSort(plugins []*Instance) ([]*Instance, error) {
	// 构建邻接表和入度表
	graph := make(map[string][]string)       // id -> 依赖它的插件列表
	inDegree := make(map[string]int)         // id -> 入度
	instanceMap := make(map[string]*Instance) // id -> instance

	for _, p := range plugins {
		id := p.Meta.ID
		instanceMap[id] = p
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
		for _, dep := range p.Meta.Dependencies {
			graph[dep] = append(graph[dep], id)
			inDegree[id]++
			// 确保依赖项也在入度表中
			if _, ok := inDegree[dep]; !ok {
				inDegree[dep] = 0
			}
		}
	}

	// BFS（Kahn 算法）
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue) // 确保确定性顺序

	var sorted []*Instance
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if inst, ok := instanceMap[curr]; ok {
			sorted = append(sorted, inst)
		}
		for _, next := range graph[curr] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(sorted) != len(plugins) {
		return nil, fmt.Errorf("插件依赖存在循环，无法启动（期望 %d，实际排序 %d）", len(plugins), len(sorted))
	}
	return sorted, nil
}

// SandboxConfig 插件沙箱配置
type SandboxConfig struct {
	MaxGoroutines int      // 单插件最大 goroutine 数（0=不限制）
	MaxMemoryMB   int64    // 单插件最大内存（MB，0=不限制）
	AllowNetwork  bool     // 是否允许网络访问
	TenantIDs     []string // 允许访问的租户（空=所有）
	Timeout       int      // 插件执行超时秒数（0=不限制）
}

// DefaultSandboxConfig 默认沙箱配置
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		MaxGoroutines: 100,
		MaxMemoryMB:   256,
		AllowNetwork:  true,
		TenantIDs:     nil, // 所有租户可用
		Timeout:       300, // 5 分钟
	}
}

// PluginPermission 插件权限绑定
type PluginPermission struct {
	PluginID    string   `json:"plugin_id"`
	PermCodes   []string `json:"perm_codes"`   // 权限编码列表 (如 plugin.hello:*)
	TenantScope string   `json:"tenant_scope"` // all / whitelist
	Whitelist   []string `json:"whitelist"`    // tenant_scope=whitelist 时的租户 ID 列表
}
