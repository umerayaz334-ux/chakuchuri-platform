package email

import (
	"html"
	"net/url"
	"strings"
)

type emailTheme struct {
	label      string
	badge      string
	icon       string
	accent     string
	accent2    string
	soft       string
	softBorder string
	headerFrom string
	headerTo   string
	pageBg     string
	ctaLabel   string
	highlight  string
	kicker     string
	details    string
}

func themeForTemplate(templateKey string) emailTheme {
	switch strings.TrimSpace(templateKey) {
	case "quote_priced":
		return emailTheme{
			label: "Quotation ready", badge: "REVIEW", icon: "📋",
			accent: "#0d9488", accent2: "#f59e0b", soft: "#f0fdfa", softBorder: "#99f6e4",
			headerFrom: "#0f766e", headerTo: "#115e59", pageBg: "#f3f7f5",
			ctaLabel: "Review quotation", highlight: "Total", details: "Quote details",
			kicker: "Formal pricing is ready in your portal.",
		}
	case "quote_accepted":
		return emailTheme{
			label: "Order confirmed", badge: "CONFIRMED", icon: "✅",
			accent: "#0f766e", accent2: "#14b8a6", soft: "#ecfdf5", softBorder: "#a7f3d0",
			headerFrom: "#0f766e", headerTo: "#134e4a", pageBg: "#f3f7f5",
			ctaLabel: "Open order", highlight: "Total", details: "Order details",
			kicker: "Production planning can begin.",
		}
	case "order_amended":
		return emailTheme{
			label: "Order updated", badge: "UPDATED", icon: "✏️",
			accent: "#0f766e", accent2: "#fbbf24", soft: "#f0fdfa", softBorder: "#99f6e4",
			headerFrom: "#134e4a", headerTo: "#0f766e", pageBg: "#f3f7f5",
			ctaLabel: "View changes", highlight: "Total", details: "Updated details",
			kicker: "Verified amendments are live on this order.",
		}
	case "order_ready":
		return emailTheme{
			label: "Ready for delivery", badge: "READY", icon: "📦",
			accent: "#059669", accent2: "#34d399", soft: "#ecfdf5", softBorder: "#6ee7b7",
			headerFrom: "#047857", headerTo: "#065f46", pageBg: "#f2f8f5",
			ctaLabel: "Open order", highlight: "Balance due", details: "Order details",
			kicker: "Manufacturing is complete and ready for handover.",
		}
	case "payment_due":
		return emailTheme{
			label: "Payment reminder", badge: "DUE NOW", icon: "💳",
			accent: "#ea580c", accent2: "#fbbf24", soft: "#fff7ed", softBorder: "#fdba74",
			headerFrom: "#c2410c", headerTo: "#9a3412", pageBg: "#faf6f2",
			ctaLabel: "Pay balance", highlight: "Balance due", details: "Payment details",
			kicker: "Clear this balance to keep production on track.",
		}
	case "payment_confirmed":
		return emailTheme{
			label: "Payment confirmed", badge: "RECEIVED", icon: "💰",
			accent: "#059669", accent2: "#34d399", soft: "#ecfdf5", softBorder: "#6ee7b7",
			headerFrom: "#047857", headerTo: "#065f46", pageBg: "#f2f8f5",
			ctaLabel: "View ledger", highlight: "Amount", details: "Payment details",
			kicker: "Your transfer is confirmed and posted.",
		}
	case "shipment_dispatched":
		return emailTheme{
			label: "Shipment dispatched", badge: "IN TRANSIT", icon: "🚚",
			accent: "#2563eb", accent2: "#38bdf8", soft: "#eff6ff", softBorder: "#93c5fd",
			headerFrom: "#1d4ed8", headerTo: "#1e3a8a", pageBg: "#f3f5f9",
			ctaLabel: "Track shipment", highlight: "Tracking", details: "Shipment details",
			kicker: "Your cargo is moving through the export channel.",
		}
	case "order_cancelled":
		return emailTheme{
			label: "Order cancelled", badge: "CLOSED", icon: "❌",
			accent: "#e11d48", accent2: "#fb7185", soft: "#fff1f2", softBorder: "#fda4af",
			headerFrom: "#be123c", headerTo: "#9f1239", pageBg: "#f8f4f5",
			ctaLabel: "View account", highlight: "Account credit", details: "Account details",
			kicker: "The charge was reversed and your ledger is updated.",
		}
	default:
		return emailTheme{
			label: "Account update", badge: "UPDATE", icon: "🔔",
			accent: "#0f766e", accent2: "#14b8a6", soft: "#ecfdf5", softBorder: "#99f6e4",
			headerFrom: "#0f766e", headerTo: "#134e4a", pageBg: "#f3f7f5",
			ctaLabel: "Open portal", highlight: "", details: "Details",
			kicker: "There is a new update on your ChakuChuri account.",
		}
	}
}

func buildEmailHTML(fromName, subject, textBody, templateKey string, values map[string]string) string {
	theme := themeForTemplate(templateKey)
	fromName = firstNonEmpty(strings.TrimSpace(fromName), "ChakuChuri")
	subject = strings.TrimSpace(subject)
	textBody = strings.ReplaceAll(strings.TrimSpace(textBody), "\r\n", "\n")
	portal := strings.TrimSpace(values["portal_url"])
	support := firstNonEmpty(strings.TrimSpace(values["support_email"]), "support@chakuchuri.pk")
	company := firstNonEmpty(strings.TrimSpace(values["company_name"]), "your account")
	customer := firstNonEmpty(strings.TrimSpace(values["customer_name"]), "there")

	paragraphs, ctaURL, _ := splitEmailBody(textBody, portal)
	ctaLabel := theme.ctaLabel

	intro := make([]string, 0)
	facts := make([]emailFact, 0)
	for _, paragraph := range paragraphs {
		if rows := parseFactBlock(paragraph); len(rows) > 0 {
			facts = append(facts, rows...)
			continue
		}
		trimmed := strings.TrimSpace(paragraph)
		if strings.EqualFold(trimmed, "ChakuChuri") || strings.EqualFold(trimmed, "ChakuChuri.pk") {
			continue
		}
		intro = append(intro, paragraph)
	}

	greeting := "Hi " + customer + ","
	bodyCopy := intro
	if len(intro) > 0 && looksLikeGreeting(intro[0]) {
		greeting = intro[0]
		bodyCopy = intro[1:]
	}
	highlightLabel, highlightValue := pickHighlight(facts, theme.highlight)

	var body strings.Builder
	body.WriteString(`<p style="margin:0 0 4px;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:15px;line-height:1.4;font-weight:700;color:#0f241c;">` + html.EscapeString(greeting) + `</p>`)
	body.WriteString(`<p style="margin:0 0 14px;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:12.5px;line-height:1.55;color:#5a6f66;">` + html.EscapeString(theme.kicker) + `</p>`)

	for _, paragraph := range bodyCopy {
		body.WriteString(`<p style="margin:0 0 10px;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:13px;line-height:1.55;color:#2a3d35;">` + html.EscapeString(paragraph) + `</p>`)
	}

	if highlightValue != "" {
		body.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:4px 0 16px;border-collapse:separate;">`)
		body.WriteString(`<tr><td style="background:` + theme.soft + `;border:1px solid ` + theme.softBorder + `;border-radius:12px;padding:14px 16px;">`)
		body.WriteString(`<div style="font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:10px;font-weight:700;letter-spacing:0.1em;text-transform:uppercase;color:` + theme.accent + `;">` + html.EscapeString(highlightLabel) + `</div>`)
		body.WriteString(`<div style="margin-top:4px;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:22px;line-height:1.15;font-weight:800;letter-spacing:-0.02em;color:#0a1f18;">` + html.EscapeString(highlightValue) + `</div>`)
		body.WriteString(`</td></tr></table>`)
	}

	if len(facts) > 0 {
		body.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 18px;border-collapse:separate;border:1px solid #dce8e2;border-radius:12px;overflow:hidden;">`)
		body.WriteString(`<tr><td colspan="2" style="padding:9px 14px;background:` + theme.headerFrom + `;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:10px;font-weight:700;letter-spacing:0.1em;text-transform:uppercase;color:#ffffff;">` + html.EscapeString(theme.details) + `</td></tr>`)
		for index, row := range facts {
			bg := "#ffffff"
			if index%2 == 1 {
				bg = "#f7faf8"
			}
			body.WriteString(`<tr>`)
			body.WriteString(`<td style="padding:9px 14px;background:` + bg + `;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:11px;font-weight:600;color:#6a7d74;width:42%;">` + html.EscapeString(row.label) + `</td>`)
			body.WriteString(`<td style="padding:9px 14px;background:` + bg + `;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:12.5px;font-weight:700;color:#12261e;text-align:right;">` + html.EscapeString(row.value) + `</td>`)
			body.WriteString(`</tr>`)
		}
		body.WriteString(`</table>`)
	}

	cta := ""
	if ctaURL != "" {
		cta = `<table role="presentation" cellpadding="0" cellspacing="0" style="margin:2px 0 6px;"><tr>` +
			`<td style="border-radius:10px;background:` + theme.accent + `;">` +
			`<a href="` + html.EscapeString(ctaURL) + `" style="display:inline-block;padding:11px 20px;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:13px;font-weight:700;color:#ffffff;text-decoration:none;">` +
			html.EscapeString(ctaLabel) + ` →</a></td></tr></table>`
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<title>` + html.EscapeString(subject) + `</title>
</head>
<body style="margin:0;padding:0;background:` + theme.pageBg + `;">
  <div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">` + html.EscapeString(theme.kicker+" "+subject) + `</div>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:` + theme.pageBg + `;padding:28px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border:1px solid #d8e4de;border-radius:16px;overflow:hidden;box-shadow:0 12px 32px rgba(15,40,32,0.08);">
          <tr>
            <td style="background:linear-gradient(135deg,` + theme.headerFrom + ` 0%,` + theme.headerTo + ` 100%);padding:20px 22px 18px;font-family:Segoe UI,Helvetica,Arial,sans-serif;">
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td>
                    <div style="font-size:11px;font-weight:800;letter-spacing:0.16em;text-transform:uppercase;color:rgba(255,255,255,0.75);">ChakuChuri</div>
                  </td>
                  <td align="right">
                    <span style="display:inline-block;padding:5px 10px;border-radius:999px;background:rgba(255,255,255,0.16);border:1px solid rgba(255,255,255,0.22);font-size:10px;font-weight:800;letter-spacing:0.06em;color:#ffffff;">` + html.EscapeString(theme.badge) + `</span>
                  </td>
                </tr>
              </table>
              <div style="margin:14px 0 4px;font-size:11px;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:rgba(255,255,255,0.82);">` + theme.icon + ` ` + html.EscapeString(theme.label) + `</div>
              <h1 style="margin:0;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:20px;line-height:1.3;font-weight:800;letter-spacing:-0.02em;color:#ffffff;">` + html.EscapeString(subject) + `</h1>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 22px 10px;font-family:Segoe UI,Helvetica,Arial,sans-serif;">
              ` + body.String() + `
              ` + cta + `
            </td>
          </tr>
          <tr>
            <td style="padding:4px 22px 18px;font-family:Segoe UI,Helvetica,Arial,sans-serif;">
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border-collapse:separate;background:#f7faf8;border:1px solid #e4ece8;border-radius:10px;">
                <tr>
                  <td style="padding:12px 14px;">
                    <p style="margin:0 0 3px;font-size:11px;line-height:1.5;color:#61756c;">Prepared for <strong style="color:#143028;">` + html.EscapeString(company) + `</strong></p>
                    <p style="margin:0;font-size:11px;line-height:1.5;color:#61756c;">` + html.EscapeString(fromName) + ` · <a href="mailto:` + html.EscapeString(support) + `" style="color:` + theme.accent + `;text-decoration:none;font-weight:700;">` + html.EscapeString(support) + `</a></p>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
        </table>
        <div style="max-width:560px;margin:14px auto 0;font-family:Segoe UI,Helvetica,Arial,sans-serif;font-size:10px;line-height:1.5;color:#85948d;text-align:center;">
          Transactional notice from ChakuChuri. Keep payment proofs inside the portal.
        </div>
      </td>
    </tr>
  </table>
</body>
</html>`
}

type emailFact struct {
	label string
	value string
}

func looksLikeGreeting(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "hi ") || strings.HasPrefix(lower, "hello ") || strings.HasPrefix(lower, "assalam") || strings.HasPrefix(lower, "dear ")
}

func pickHighlight(facts []emailFact, preferred string) (string, string) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		for _, fact := range facts {
			if strings.EqualFold(fact.label, preferred) {
				return fact.label, fact.value
			}
		}
	}
	priority := []string{"Balance due", "Amount", "Total", "Tracking", "Account credit", "Kul raqam", "Baqaya", "Raqam"}
	for _, label := range priority {
		for _, fact := range facts {
			if strings.EqualFold(fact.label, label) {
				return fact.label, fact.value
			}
		}
	}
	return "", ""
}

func splitEmailBody(textBody, portal string) (paragraphs []string, ctaURL, ctaLabel string) {
	chunks := strings.Split(textBody, "\n\n")
	kept := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		lines := strings.Split(chunk, "\n")
		if len(lines) == 1 {
			line := strings.TrimSpace(lines[0])
			if extractedURL, label, ok := extractCTA(line, portal); ok {
				ctaURL = extractedURL
				ctaLabel = label
				continue
			}
		}
		kept = append(kept, chunk)
	}
	if ctaURL == "" && portal != "" {
		ctaURL = strings.TrimRight(portal, "/")
		ctaLabel = "Open portal"
	}
	return kept, ctaURL, ctaLabel
}

func extractCTA(line, portal string) (string, string, bool) {
	line = strings.TrimSpace(line)
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return line, "Open portal", true
	}
	prefixes := []string{"open ", "view ", "track ", "review ", "pay ", "portal: ", "link: "}
	for _, prefix := range prefixes {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		rest := strings.TrimSpace(line[len(prefix):])
		restLower := strings.ToLower(rest)
		if strings.HasPrefix(restLower, "http://") || strings.HasPrefix(restLower, "https://") {
			return rest, niceCTALabel(prefix), true
		}
		if portal != "" && strings.HasPrefix(rest, "/") {
			return strings.TrimRight(portal, "/") + rest, niceCTALabel(prefix), true
		}
	}
	if parsed, err := url.Parse(line); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return line, "Open portal", true
	}
	return "", "", false
}

func niceCTALabel(prefix string) string {
	label := strings.TrimSpace(strings.TrimSuffix(prefix, ":"))
	if label == "" {
		return "Open portal"
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

func parseFactBlock(paragraph string) []emailFact {
	lines := strings.Split(paragraph, "\n")
	if len(lines) < 2 {
		return nil
	}
	rows := make([]emailFact, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil
		}
		label := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if label == "" || value == "" || strings.EqualFold(label, "http") || strings.EqualFold(label, "https") {
			return nil
		}
		if strings.Contains(strings.ToLower(label), "http") {
			return nil
		}
		rows = append(rows, emailFact{label: label, value: value})
	}
	if len(rows) < 2 {
		return nil
	}
	return rows
}
