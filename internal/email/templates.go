package email

import (
	"fmt"
	"strings"
)

// OrderItem represents an item in an order for email purposes
type OrderItem struct {
	ProductID string
	Name      string
	Quantity  int
	Price     int
}

// BuildOrderConfirmationBody builds the HTML body for order confirmation email
func BuildOrderConfirmationBody(orderID string, total int, items []OrderItem) string {
	var itemsHTML strings.Builder
	for _, item := range items {
		name := item.Name
		if name == "" {
			name = item.ProductID
		}
		itemsHTML.WriteString(fmt.Sprintf(
			`<tr>
				<td style="padding: 12px; border-bottom: 1px solid #eee;">%s</td>
				<td style="padding: 12px; border-bottom: 1px solid #eee; text-align: center;">%d</td>
				<td style="padding: 12px; border-bottom: 1px solid #eee; text-align: right;">¥%s</td>
				<td style="padding: 12px; border-bottom: 1px solid #eee; text-align: right;">¥%s</td>
			</tr>`,
			name,
			item.Quantity,
			formatNumber(item.Price),
			formatNumber(item.Price*item.Quantity),
		))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
	<div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 30px; border-radius: 10px 10px 0 0;">
		<h1 style="color: white; margin: 0; font-size: 24px;">ご注文ありがとうございます</h1>
	</div>

	<div style="background: #fff; padding: 30px; border: 1px solid #eee; border-top: none; border-radius: 0 0 10px 10px;">
		<p style="margin-top: 0;">この度はご注文いただき、誠にありがとうございます。</p>

		<div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin: 20px 0;">
			<p style="margin: 0; font-size: 14px; color: #666;">注文番号</p>
			<p style="margin: 5px 0 0 0; font-size: 18px; font-weight: bold; font-family: monospace;">%s</p>
		</div>

		<h2 style="font-size: 18px; border-bottom: 2px solid #667eea; padding-bottom: 10px;">ご注文内容</h2>

		<table style="width: 100%%; border-collapse: collapse; margin: 20px 0;">
			<thead>
				<tr style="background: #f8f9fa;">
					<th style="padding: 12px; text-align: left; font-weight: 600;">商品名</th>
					<th style="padding: 12px; text-align: center; font-weight: 600;">数量</th>
					<th style="padding: 12px; text-align: right; font-weight: 600;">単価</th>
					<th style="padding: 12px; text-align: right; font-weight: 600;">小計</th>
				</tr>
			</thead>
			<tbody>
				%s
			</tbody>
		</table>

		<div style="text-align: right; padding: 20px; background: #f8f9fa; border-radius: 5px;">
			<span style="font-size: 14px; color: #666;">合計金額</span>
			<span style="font-size: 24px; font-weight: bold; color: #667eea; margin-left: 10px;">¥%s</span>
		</div>

		<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">

		<p style="font-size: 12px; color: #999; margin-bottom: 0;">
			このメールは自動送信されています。ご不明な点がございましたら、サポートまでお問い合わせください。
		</p>
	</div>
</body>
</html>`, orderID, itemsHTML.String(), formatNumber(total))
}

// formatNumber formats a number with comma separators
func formatNumber(n int) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result strings.Builder
	remainder := len(str) % 3
	if remainder > 0 {
		result.WriteString(str[:remainder])
		if len(str) > remainder {
			result.WriteString(",")
		}
	}

	for i := remainder; i < len(str); i += 3 {
		result.WriteString(str[i : i+3])
		if i+3 < len(str) {
			result.WriteString(",")
		}
	}

	return result.String()
}
