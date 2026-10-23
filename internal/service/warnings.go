package service

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func committedMaintenance(operation string, target domain.TargetKind, result any, err error) error {
	if err == nil {
		return nil
	}
	details, _ := json.Marshal(map[string]any{"committed": true, "operation": operation, "target": target, "result": result, "warning": publicError(err).Error()})
	text := string(details)
	return &domain.AppError{Code: "committed_with_warning", Message: "configuration and database changes committed; maintenance failed", Details: &text}
}

func committedWarning(operation string, target domain.TargetKind, result any, err error) error {
	return committedMaintenance(operation, target, result, err)
}
func isCommitted(err error) bool {
	var app *domain.AppError
	return errors.As(err, &app) && app.Code == "committed_with_warning"
}
