package app

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"time"
)

func quotaKey(site domain.SiteDTO) string {
	return site.ID + "\x00" + value(site.ActiveAPIKeyID) + "\x00" + site.BaseURL + "\x00" + site.QuotaRevision
}
func (s *Shell) recordQuota(site domain.SiteDTO, q domain.SiteQuota) {
	key := quotaKey(site)
	s.Quotas[key] = q
	if q.Status == "available" {
		s.QuotaSuccess[key] = q
	}
}
func (s *Shell) quotaAmount(n *float64, unit string) string {
	number, unit := s.quotaParts(n, unit)
	if unit == "" {
		return number
	}
	return number + " " + unit
}
func (s *Shell) quotaParts(n *float64, unit string) (string, string) {
	if n == nil {
		return s.text("unknown"), ""
	}
	if unit == "" {
		unit = "USD"
	}
	return fmt.Sprintf("%.4g", *n), unit
}
func (s *Shell) quotaPanel(c *ui.Context, site domain.SiteDTO) {
	Panel(c).Grow(1).Children(func() {
		PanelTitle(c, icWallet, TintGreen, s.text("quota"), "", false, func() {
			if BusyPillButton(c, icRefresh, s.text("fetch_quota"), false, s.Busy).Clicked() {
				s.probeQuota(site)
			}
		})
		s.quotaView(c, site)
	})
}
func (s *Shell) quotaView(c *ui.Context, site domain.SiteDTO) {
	t := c.Theme()
	key := quotaKey(site)
	attempt, ok := s.Quotas[key]
	if !ok {
		ui.Text(c, s.text("quota_not_fetched")).FontSize(12).TextColor(t.TextMuted).Wrap()
		return
	}
	q := attempt
	if q.Status != "available" {
		if successful, ok := s.QuotaSuccess[key]; ok {
			q = successful
		}
	}
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		if q.Status == "available" {
			number, unit := s.quotaParts(q.RemainingUSD, value(q.Unit))
			if q.Unlimited {
				number, unit = s.text("unlimited"), ""
			}
			BigValue(c, s.text("quota_remaining"), number, unit, TintGreen.Text(t))
			if !q.Unlimited && q.RemainingUSD != nil && q.TotalUSD != nil && *q.TotalUSD > 0 {
				Bar(c, *q.RemainingUSD / *q.TotalUSD, TintGreen.C(t))
			}
			Metrics(c, Metric{Label: s.text("quota_used"), Value: s.quotaAmount(q.UsedUSD, value(q.Unit))}, Metric{Label: s.text("quota_total"), Value: s.quotaAmount(q.TotalUSD, value(q.Unit))})
			if q.ExpiresAt != nil && *q.ExpiresAt > 0 && time.Unix(*q.ExpiresAt, 0).Year() < 2099 {
				label, col := s.text("expires"), t.TextMuted
				if *q.ExpiresAt <= time.Now().Unix() {
					label, col = s.text("quota_expired"), t.Danger
				}
				ui.Text(c, label+": "+time.Unix(*q.ExpiresAt, 0).Format("2006-01-02")).FontSize(11).TextColor(col)
			}
			if q.FetchedAt > 0 {
				ui.Text(c, s.text("updated")+": "+time.UnixMilli(q.FetchedAt).Format("2006-01-02 15:04")).FontSize(11).TextColor(t.TextMuted)
			}
		}
		if attempt.Status != "available" {
			label, known := s.Config.Messages["quota_"+string(attempt.Status)]
			if !known {
				label = s.text("operation_failed")
			}
			ui.Row(c).Children(func() { DotPill(c, label, t.Warning) })
			if attempt.Error != nil {
				ui.Text(c, s.safeErrorDetail(*attempt.Error)).FontSize(11).TextColor(t.TextMuted).MaxLines(3)
			}
		}
	})
}
