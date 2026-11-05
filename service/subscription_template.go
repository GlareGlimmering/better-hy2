package service

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"h-ui/dao"
	"h-ui/model/constant"
	"h-ui/model/entity"
	"h-ui/model/vo"
)

const (
	defaultNodeNameTemplate = "Hysteria2"
	defaultServerTemplate   = "{SERVER_IP}"
	permanentExpireTime     = int64(253370736000000)
)

var templateVariablePattern = regexp.MustCompile(`\{[A-Z0-9_]+\}`)

var subscriptionVariables = []string{
	"{SERVER_IP}", "{SERVER_IPV6}", "{USERNAME}", "{DATA_USAGE}",
	"{DATA_LEFT}", "{DATA_LIMIT}", "{DAYS_LEFT}", "{EXPIRE_DATE}",
	"{JALALI_EXPIRE_DATE}", "{TIME_LEFT}", "{STATUS_TEXT}",
	"{STATUS_EMOJI}", "{PROTOCOL}", "{TRANSPORT}",
}

type subscriptionNode struct {
	Name    string
	Address string
}

func SubscriptionTemplateSettings() (vo.SubscriptionTemplateVo, error) {
	name, address, err := subscriptionTemplateValues()
	if err != nil {
		return vo.SubscriptionTemplateVo{}, err
	}
	result := vo.SubscriptionTemplateVo{
		NodeNameTemplate: name, ServerAddressTemplate: address,
		Variables: append([]string(nil), subscriptionVariables...),
	}
	result.ServerIPv4, err = NodeAddress()
	if err != nil {
		result.ServerIPv4Error = err.Error()
	}
	result.ServerIPv6, err = NodeAddressIPv6()
	if err != nil {
		result.ServerIPv6Error = err.Error()
	}
	accounts, listErr := dao.ListAccount(nil, nil)
	if listErr == nil && len(accounts) > 0 {
		preview, previewErr := renderSubscriptionNode(name, address, accounts[0], time.Now())
		if previewErr != nil {
			result.PreviewError = previewErr.Error()
		} else {
			result.PreviewName, result.PreviewAddress = preview.Name, preview.Address
		}
	}
	return result, nil
}

func UpdateSubscriptionTemplate(name, address string) error {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	if err := validateTemplate(name, false); err != nil {
		return fmt.Errorf("node name template: %w", err)
	}
	if err := validateTemplate(address, true); err != nil {
		return fmt.Errorf("server address template: %w", err)
	}
	accounts, err := dao.ListAccount(nil, nil)
	if err != nil {
		return err
	}
	if len(accounts) > 0 {
		if _, err := renderSubscriptionNode(name, address, accounts[0], time.Now()); err != nil {
			return err
		}
	}
	keyName, keyAddress := constant.SubscriptionNodeNameTemplate, constant.SubscriptionServerTemplate
	nameRemark, addressRemark := "Subscription node name template", "Subscription server address template"
	return dao.UpsertConfig([]entity.Config{
		{Key: &keyName, Value: &name, Remark: &nameRemark},
		{Key: &keyAddress, Value: &address, Remark: &addressRemark},
	})
}

func ResetSubscriptionTemplate() error {
	return UpdateSubscriptionTemplate(defaultNodeNameTemplate, defaultServerTemplate)
}

func subscriptionTemplateValues() (string, string, error) {
	configs, err := dao.ListConfig("key in ?", []string{constant.SubscriptionNodeNameTemplate, constant.SubscriptionServerTemplate})
	if err != nil {
		return "", "", err
	}
	name, address := defaultNodeNameTemplate, defaultServerTemplate
	for _, config := range configs {
		if config.Key == nil || config.Value == nil {
			continue
		}
		switch *config.Key {
		case constant.SubscriptionNodeNameTemplate:
			name = *config.Value
		case constant.SubscriptionServerTemplate:
			address = *config.Value
		}
	}
	return name, address, nil
}

func subscriptionNodeForAccount(account entity.Account) (subscriptionNode, error) {
	name, address, err := subscriptionTemplateValues()
	if err != nil {
		return subscriptionNode{}, err
	}
	return renderSubscriptionNode(name, address, account, time.Now())
}

func renderSubscriptionNode(nameTemplate, addressTemplate string, account entity.Account, now time.Time) (subscriptionNode, error) {
	if err := validateTemplate(nameTemplate, false); err != nil {
		return subscriptionNode{}, fmt.Errorf("node name template: %w", err)
	}
	if err := validateTemplate(addressTemplate, true); err != nil {
		return subscriptionNode{}, fmt.Errorf("server address template: %w", err)
	}
	combinedTemplate := nameTemplate + addressTemplate
	if strings.Contains(combinedTemplate, "{SERVER_IP}") {
		if _, err := NodeAddress(); err != nil {
			return subscriptionNode{}, fmt.Errorf("resolve {SERVER_IP}: %w", err)
		}
	}
	if strings.Contains(combinedTemplate, "{SERVER_IPV6}") {
		if _, err := NodeAddressIPv6(); err != nil {
			return subscriptionNode{}, fmt.Errorf("resolve {SERVER_IPV6}: %w", err)
		}
	}
	values, err := subscriptionTemplateValuesForAccount(account, now)
	if err != nil {
		return subscriptionNode{}, err
	}
	name := replaceTemplateVariables(nameTemplate, values)
	address := replaceTemplateVariables(addressTemplate, values)
	if strings.TrimSpace(name) == "" {
		return subscriptionNode{}, errors.New("rendered node name is empty")
	}
	if err := validateServerHost(address); err != nil {
		return subscriptionNode{}, fmt.Errorf("rendered server address: %w", err)
	}
	return subscriptionNode{Name: name, Address: address}, nil
}

func validateTemplate(value string, address bool) error {
	if value == "" {
		return errors.New("must not be empty")
	}
	allowed := make(map[string]bool, len(subscriptionVariables))
	for _, variable := range subscriptionVariables {
		allowed[variable] = true
	}
	for _, variable := range templateVariablePattern.FindAllString(value, -1) {
		if !allowed[variable] {
			return fmt.Errorf("unsupported variable %s", variable)
		}
		if address && variable != "{SERVER_IP}" && variable != "{SERVER_IPV6}" {
			return fmt.Errorf("variable %s is not allowed in a server address", variable)
		}
	}
	if strings.Contains(value, "{") || strings.Contains(value, "}") {
		stripped := templateVariablePattern.ReplaceAllString(value, "")
		if strings.Contains(stripped, "{") || strings.Contains(stripped, "}") {
			return errors.New("malformed variable placeholder")
		}
	}
	if address && len(templateVariablePattern.FindAllString(value, -1)) == 0 {
		return validateServerHost(value)
	}
	return nil
}

func validateServerHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return errors.New("host is empty")
	}
	if strings.ContainsAny(host, "/?#@[]") {
		return errors.New("host must not contain a scheme, path, port, or brackets")
	}
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}
	if len(host) > 253 {
		return errors.New("hostname is too long")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return errors.New("invalid hostname")
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' {
				return errors.New("invalid hostname")
			}
		}
	}
	return nil
}

func subscriptionTemplateValuesForAccount(account entity.Account, now time.Time) (map[string]string, error) {
	ipv4, ipv4Err := NodeAddress()
	ipv6, ipv6Err := NodeAddressIPv6()
	used := valueInt64(account.Download) + valueInt64(account.Upload)
	limit := valueInt64(account.Quota)
	expireMillis := valueInt64(account.ExpireTime)
	expire := time.UnixMilli(expireMillis).UTC()
	remaining := expire.Sub(now.UTC())
	if remaining < 0 {
		remaining = 0
	}
	left, limitText := "Unlimited", "Unlimited"
	if limit >= 0 {
		leftBytes := limit - used
		if leftBytes < 0 {
			leftBytes = 0
		}
		left, limitText = formatTemplateBytes(leftBytes), formatTemplateBytes(limit)
	}
	status, emoji := subscriptionAccountStatus(account, now)
	values := map[string]string{
		"{USERNAME}": valueString(account.Username), "{DATA_USAGE}": formatTemplateBytes(used),
		"{DATA_LEFT}": left, "{DATA_LIMIT}": limitText,
		"{DAYS_LEFT}":          fmt.Sprintf("%d", int64(remaining/(24*time.Hour))),
		"{EXPIRE_DATE}":        expire.Format("2006-01-02"),
		"{JALALI_EXPIRE_DATE}": formatJalaliDate(expire),
		"{TIME_LEFT}":          formatTimeLeft(remaining), "{STATUS_TEXT}": status,
		"{STATUS_EMOJI}": emoji, "{PROTOCOL}": "Hysteria2", "{TRANSPORT}": "QUIC",
	}
	if expireMillis >= permanentExpireTime {
		values["{DAYS_LEFT}"] = "Permanent"
		values["{EXPIRE_DATE}"] = "Permanent"
		values["{JALALI_EXPIRE_DATE}"] = "Permanent"
		values["{TIME_LEFT}"] = "Permanent"
	}
	if ipv4Err == nil {
		values["{SERVER_IP}"] = ipv4
	}
	if ipv6Err == nil {
		values["{SERVER_IPV6}"] = ipv6
	}
	if _, needed := values["{SERVER_IP}"]; !needed && ipv4Err != nil {
		values["{SERVER_IP}"] = ""
	}
	if _, needed := values["{SERVER_IPV6}"]; !needed && ipv6Err != nil {
		values["{SERVER_IPV6}"] = ""
	}
	return values, nil
}

func replaceTemplateVariables(template string, values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		template = strings.ReplaceAll(template, key, values[key])
	}
	return template
}

func valueInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
func valueString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatTemplateBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	value := float64(bytes)
	unit := "B"
	for _, candidate := range units {
		value /= 1024
		unit = candidate
		if value < 1024 {
			break
		}
	}
	return fmt.Sprintf("%.2f %s", value, unit)
}

func formatTimeLeft(duration time.Duration) string {
	if duration <= 0 {
		return "0m"
	}
	days := duration / (24 * time.Hour)
	duration %= 24 * time.Hour
	hours := duration / time.Hour
	duration %= time.Hour
	minutes := duration / time.Minute
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func subscriptionAccountStatus(account entity.Account, now time.Time) (string, string) {
	if valueInt64(account.Deleted) != 0 {
		return "Disabled", "⛔"
	}
	if valueInt64(account.KickUtilTime) >= now.UnixMilli() {
		return "Temporarily disabled", "⏸️"
	}
	if valueInt64(account.ExpireTime) <= now.UnixMilli() {
		return "Expired", "⌛"
	}
	limit, used := valueInt64(account.Quota), valueInt64(account.Download)+valueInt64(account.Upload)
	if limit >= 0 && used >= limit {
		return "Quota exhausted", "⚠️"
	}
	return "Active", "✅"
}

// formatJalaliDate converts a Gregorian UTC date using the standard 2820-year
// Persian calendar arithmetic. It is intentionally date-only and timezone-stable.
func formatJalaliDate(date time.Time) string {
	gy, gm, gd := date.Date()
	gdm := []int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	gy2 := gy
	if gm > 2 {
		gy2++
	}
	days := 355666 + 365*gy + (gy2+3)/4 - (gy2+99)/100 + (gy2+399)/400 + gd + gdm[gm-1]
	jy := -1595 + 33*(days/12053)
	days %= 12053
	jy += 4 * (days / 1461)
	days %= 1461
	if days > 365 {
		jy += (days - 1) / 365
		days = (days - 1) % 365
	}
	jm, jd := 0, 0
	if days < 186 {
		jm = 1 + days/31
		jd = 1 + days%31
	} else {
		jm = 7 + (days-186)/30
		jd = 1 + (days-186)%30
	}
	return fmt.Sprintf("%04d-%02d-%02d", jy, jm, jd)
}
