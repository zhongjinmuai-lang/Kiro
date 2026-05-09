// Package docs 由 `swag init` 生成的 Swagger 注册包
// 实际生产请执行： swag init -g cmd/api-server/main.go -o backend/docs --ot go,yaml
// 本文件为占位骨架，确保 `import _ "xxx/backend/docs"` 可编译通过。
package docs

import "github.com/swaggo/swag"

// SwaggerInfo 基本信息（运行时由 swag 工具替换）
var SwaggerInfo = &swag.Spec{
	Version:          "1.0.0",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{"http", "https"},
	Title:            "MU Framework API",
	Description:      "MU自研全能智能体主体框架 — 统一API规范（OpenAPI 3.1）",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  defaultTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}

// defaultTemplate 基础 Swagger JSON 模板（极简占位，swag 工具会完整替换）
const defaultTemplate = `{
  "swagger": "2.0",
  "info": {
    "title": "{{.Title}}",
    "description": "{{.Description}}",
    "version": "{{.Version}}"
  },
  "host": "{{.Host}}",
  "basePath": "{{.BasePath}}",
  "schemes": {{ marshal .Schemes }},
  "paths": {
    "/health": {
      "get": {
        "summary": "健康检查",
        "tags": ["System"],
        "responses": { "200": { "description": "OK" } }
      }
    }
  }
}`
