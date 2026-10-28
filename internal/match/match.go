// Package match 实现 MQTT 主题过滤器匹配，以及 ACL 引擎用于客户端 id
// 和用户名的简单 glob 匹配。
package match

import "strings"

// Topic 判断 topicName 是否被一个可能含单层 (+) 和多级 (#) 通配符的
// MQTT 主题过滤器匹配，遵循协议规范。
func Topic(filter, topicName string) bool {
	if filter == topicName {
		return true
	}
	// $SYS 类主题只能被同样以 $ 开头的过滤器匹配。
	if strings.HasPrefix(topicName, "$") && !strings.HasPrefix(filter, "$") {
		return false
	}
	fs := strings.Split(filter, "/")
	ts := strings.Split(topicName, "/")
	for i, fseg := range fs {
		switch fseg {
		case "#":
			// 多级通配符必须是最后一段；匹配剩余全部。
			return true
		case "+":
			if i >= len(ts) {
				return false
			}
		default:
			if i >= len(ts) || fseg != ts[i] {
				return false
			}
		}
	}
	return len(fs) == len(ts)
}

// Glob 将值与模式匹配：模式可以以 '*' 结尾表示前缀，或使用 '+' 作为
// 单字符占位。空模式匹配任意值。
func Glob(pattern, value string) bool {
	if pattern == "" {
		return true
	}
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	if len(pattern) != len(value) {
		return false
	}
	for i := range pattern {
		if pattern[i] == '+' {
			continue
		}
		if pattern[i] != value[i] {
			return false
		}
	}
	return true
}
