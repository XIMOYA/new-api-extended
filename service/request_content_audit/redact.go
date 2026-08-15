// service/request_content_audit/redact.go
// 非 Root 管理员查看请求内容时的最小化脱敏规则，避免返回密钥、认证信息和个人联系方式。
package request_content_audit

import (
	"regexp"
	"strings"
)

var (
	bearerSecretPattern = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	apiKeyPattern       = regexp.MustCompile(`(?i)\b(sk-[A-Za-z0-9_-]{8,}|key-[A-Za-z0-9_-]{8,})\b`)
	jwtPattern          = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	emailPattern        = regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`)
	phonePattern        = regexp.MustCompile(`\b(\+?1[ -]?)?([2-9][0-9]{2})[ -]?[0-9]{3}[ -]?[0-9]{4}\b`)
)

func RedactValue(value any) any {
	return redactValue(value, false)
}

// RedactValueLimited applies the same redaction rules with bounded recursion for derived views.
func RedactValueLimited(value any, maxDepth, maxNodes int) any {
	if maxDepth < 1 {
		maxDepth = 1
	}
	if maxNodes < 1 {
		maxNodes = 1
	}
	budget := &redactBudget{remaining: maxNodes}
	return redactValueLimited(value, false, 0, maxDepth, budget)
}

type redactBudget struct {
	remaining int
}

func (budget *redactBudget) take() bool {
	if budget.remaining <= 0 {
		return false
	}
	budget.remaining--
	return true
}

func redactValueLimited(value any, sensitiveKey bool, depth, maxDepth int, budget *redactBudget) any {
	if depth > maxDepth || !budget.take() {
		return "[REDACTED_NESTED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = redactValueLimited(child, isSensitiveKey(key), depth+1, maxDepth, budget)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			result = append(result, redactValueLimited(child, sensitiveKey, depth+1, maxDepth, budget))
		}
		return result
	case string:
		if sensitiveKey {
			return "[REDACTED]"
		}
		return RedactText(typed)
	default:
		return value
	}
}

func RedactText(value string) string {
	value = bearerSecretPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = apiKeyPattern.ReplaceAllString(value, "[REDACTED_KEY]")
	value = jwtPattern.ReplaceAllString(value, "[REDACTED_JWT]")
	value = emailPattern.ReplaceAllString(value, "[REDACTED_EMAIL]")
	value = phonePattern.ReplaceAllString(value, "[REDACTED_PHONE]")
	return value
}

func redactValue(value any, sensitiveKey bool) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = redactValue(child, isSensitiveKey(key))
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redactValue(child, sensitiveKey)
		}
		return result
	case string:
		if sensitiveKey {
			return "[REDACTED]"
		}
		return RedactText(typed)
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{
		"authorization",
		"access_token",
		"refresh_token",
		"api_key",
		"apikey",
		"secret",
		"password",
		"cookie",
		"credential",
		"private_key",
		"file_name",
		"filename",
	} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
