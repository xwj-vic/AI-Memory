package memory

import (
	"ai-memory/pkg/logger"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// NotifyConfig 通知配置
type NotifyConfig struct {
	// Webhook配置
	WebhookEnabled bool
	WebhookURL     string
	WebhookTimeout time.Duration

	// 邮件配置
	EmailEnabled  bool
	EmailSMTPHost string
	EmailSMTPPort int
	EmailUsername string
	EmailPassword string
	EmailFrom     string
	EmailTo       []string
	EmailUseTLS   bool

	// 通知级别
	NotifyLevels map[AlertLevel]bool
}

// AlertNotifier 告警通知器
type AlertNotifier struct {
	config *NotifyConfig
}

// NewAlertNotifier 创建通知器
func NewAlertNotifier(config *NotifyConfig) *AlertNotifier {
	return &AlertNotifier{
		config: config,
	}
}

// Notify 发送通知
func (an *AlertNotifier) Notify(alert *Alert) {
	// 检查是否需要通知该级别的告警
	if !an.config.NotifyLevels[alert.Level] {
		return
	}

	// 并发发送各种通知（不阻塞）
	if an.config.WebhookEnabled && an.config.WebhookURL != "" {
		go an.sendWebhook(alert)
	}

	if an.config.EmailEnabled && len(an.config.EmailTo) > 0 {
		go an.sendEmail(alert)
	}
}

// sendWebhook 发送Webhook通知
func (an *AlertNotifier) sendWebhook(alert *Alert) {
	// 构建通用的Webhook payload
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": an.formatMarkdown(alert),
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal webhook payload", err)
		return
	}

	client := &http.Client{
		Timeout: an.config.WebhookTimeout,
	}

	resp, err := client.Post(an.config.WebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		logger.Error("Failed to send webhook", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.System("✅ Webhook notification sent", "rule", alert.Rule, "level", alert.Level)
	} else {
		logger.Error("Webhook returned non-2xx status", fmt.Errorf("status: %d", resp.StatusCode))
	}
}

// formatMarkdown 格式化为Markdown消息
func (an *AlertNotifier) formatMarkdown(alert *Alert) string {
	emoji := map[AlertLevel]string{
		AlertLevelError:   "🔴",
		AlertLevelWarning: "🟡",
		AlertLevelInfo:    "🔵",
	}

	var metadataStr string
	if len(alert.Metadata) > 0 {
		metadataStr = "\n\n**详情**:\n"
		for k, v := range alert.Metadata {
			metadataStr += fmt.Sprintf("- %s: %v\n", k, v)
		}
	}

	return fmt.Sprintf(
		"## %s AI-Memory 告警通知\n\n"+
			"**级别**: %s\n"+
			"**规则**: %s\n"+
			"**消息**: %s\n"+
			"**时间**: %s"+
			"%s",
		emoji[alert.Level],
		alert.Level,
		alert.Rule,
		alert.Message,
		alert.Timestamp.Format("2006-01-02 15:04:05"),
		metadataStr,
	)
}

// sendEmail 发送邮件通知
func (an *AlertNotifier) sendEmail(alert *Alert) {
	subject := fmt.Sprintf("[%s 告警] %s", alert.Level, alert.Rule)
	body := an.formatEmailBody(alert)

	// 构建邮件内容
	message := []byte(fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n"+
			"\r\n"+
			"%s",
		an.config.EmailFrom,
		strings.Join(an.config.EmailTo, ","),
		subject,
		body,
	))

	// SMTP认证
	auth := smtp.PlainAuth(
		"",
		an.config.EmailUsername,
		an.config.EmailPassword,
		an.config.EmailSMTPHost,
	)

	addr := fmt.Sprintf("%s:%d", an.config.EmailSMTPHost, an.config.EmailSMTPPort)

	var err error
	if an.config.EmailUseTLS {
		// 使用TLS加密
		err = an.sendMailTLS(addr, auth, an.config.EmailFrom, an.config.EmailTo, message)
	} else {
		// 明文或STARTTLS
		err = smtp.SendMail(addr, auth, an.config.EmailFrom, an.config.EmailTo, message)
	}

	if err != nil {
		logger.Error("Failed to send email", err)
		return
	}

	logger.System("✅ Email notification sent", "rule", alert.Rule, "to", strings.Join(an.config.EmailTo, ","))
}

// sendMailTLS 使用TLS发送邮件
func (an *AlertNotifier) sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// 建立TLS连接
	tlsconfig := &tls.Config{
		ServerName: an.config.EmailSMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsconfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, an.config.EmailSMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	// 认证
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	// 发送
	if err = client.Mail(from); err != nil {
		return err
	}

	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// formatEmailBody 格式化邮件正文
func (an *AlertNotifier) formatEmailBody(alert *Alert) string {
	var metadataStr string
	if len(alert.Metadata) > 0 {
		metadataStr = "\n\n详细信息:\n"
		for k, v := range alert.Metadata {
			metadataStr += fmt.Sprintf("  %s: %v\n", k, v)
		}
	}

	return fmt.Sprintf(
		"AI-Memory 监控系统告警通知\n\n"+
			"告警级别: %s\n"+
			"告警规则: %s\n"+
			"告警消息: %s\n"+
			"触发时间: %s\n"+
			"%s\n"+
			"---\n"+
			"本邮件由 AI-Memory 监控系统自动发送，请勿回复。\n",
		alert.Level,
		alert.Rule,
		alert.Message,
		alert.Timestamp.Format("2006-01-02 15:04:05"),
		metadataStr,
	)
}
