package app

import (
	"errors"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func (s *Shell) errorText(err error) string {
	var app *domain.AppError
	if errors.As(err, &app) {
		if text, ok := s.Config.Messages["error_"+app.Code]; ok {
			return text + " [" + app.Code + "]"
		}
		return s.text("operation_failed") + " [" + app.Code + "]"
	}
	return s.text("operation_failed")
}

func (s *Shell) technicalError(c *ui.Context) {
	detail := s.ErrorDetail
	if detail == "" {
		detail = s.WarningDetail
	}
	if detail == "" {
		return
	}
	disclosure(c, "error_details", icCode, s.text("technical_details"), &s.ErrorOpen, func() {
		ui.Scroll(c).MaxHeight(160).FillWidth().Radius(10).Background(pal(c.Theme()).Inset).Children(func() {
			mono(c, detail).Wrap().Selectable().Padding(10)
		})
	})
}
