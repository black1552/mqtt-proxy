package main

import "embed"

// distFS 内嵌已编译好的 AntD/React 管理后台。构建流程会在 admin/ 下执行
// `npm run build`，因此在 `go build` 前 admin/dist 已被填充。
// 仓库中保留了一个占位页，使得没有前端时二进制仍能编译。
//
//go:embed all:admin/dist
var distFS embed.FS
