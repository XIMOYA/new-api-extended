package operation_setting

import "strings"

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

// StreamHandoffEnabled 控制「流式接力续写」：上游在流中途失败（典型是余额耗尽）时，
// 是否让接替的渠道在同一条 SSE 连接里从断点继续生成。
//
// 默认关闭。它会改变两件事，需要管理员明确知晓后再开启：
//   - 已输出内容会作为 prompt 重新发给新上游，这部分 prompt token 会重复计费；
//   - 断点处可能出现文风或格式跳变，回答连贯性不如一次生成。
var StreamHandoffEnabled = false

// AutomaticDisableKeywords 命中即自动禁用渠道的错误文本。
// 匹配由 service.AcSearch 完成，模式串在建机时统一小写化，因此这里的大小写不影响匹配。
//
// 这份表同时驱动两件事：渠道自动禁用，以及 IsUpstreamExhaustedError 判定的
// 「上游自己没额度」——后者会让请求即使拿到 400 这类默认不重试的状态码也继续换渠道。
// 因此新增词必须足够特指，避免把普通参数错误误判成余额问题。
var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
	// 以下为各家上游表达「余额/额度耗尽」的其他常见文案。
	// OpenAI / Azure
	"insufficient_quota",
	"billing_hard_limit_reached",
	"billing hard limit has been reached",
	// 通用中转 / 国内厂商
	"insufficient balance",
	"insufficient_user_quota",
	"quota exhausted",
	"account balance is insufficient",
	"余额不足",
	"额度不足",
	"配额不足",
	"欠费",
}

func AutomaticDisableKeywordsToString() string {
	return strings.Join(AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(s string) {
	AutomaticDisableKeywords = []string{}
	ak := strings.Split(s, "\n")
	for _, k := range ak {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			AutomaticDisableKeywords = append(AutomaticDisableKeywords, k)
		}
	}
}
