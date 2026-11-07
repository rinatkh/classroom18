// Package logging настраивает стандартный log/slog для всех transport adapters.
// Business UseCases не зависят от logger и возвращают ошибки вызывающему коду.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// В slog нет встроенного Fatal. Добавляем имя уровня и отдельно завершаем процесс.
const LevelFatal = slog.Level(12)

// New создаёт logger без глобальных изменений. Это удобно для тестов.
func New(output io.Writer, service, levelName, format string) (*slog.Logger, error) {
	var level slog.Level
	switch strings.ToUpper(strings.TrimSpace(levelName)) {
	case "", "INFO":
		level = slog.LevelInfo
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	case "FATAL":
		level = LevelFatal
	default:
		return nil, errors.New("LOG_LEVEL: use DEBUG, INFO, WARN, ERROR or FATAL")
	}
	options := &slog.HandlerOptions{Level: level, ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) == 0 && a.Key == slog.LevelKey && a.Value.Any() == LevelFatal {
			a.Value = slog.StringValue("FATAL")
		}
		return a
	}}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		handler = slog.NewTextHandler(output, options)
	case "json":
		handler = slog.NewJSONHandler(output, options)
	default:
		return nil, errors.New("LOG_FORMAT: use text or json")
	}
	return slog.New(handler).With("service", service), nil
}

// Configure вызывается один раз в main. Все slog.Debug/Info/Error используют его.
// При ошибке config оставляем рабочий INFO logger, чтобы FATAL был виден.
func Configure(service string) error {
	logger, err := New(os.Stderr, service, os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FORMAT"))
	if err != nil {
		logger, _ = New(os.Stderr, service, "INFO", "text")
	}
	slog.SetDefault(logger)
	return err
}

// ErrorType помогает диагностике без raw error text: driver errors могут содержать
// SQL values, email, password hash или иные данные. Payload/credentials не пишем.
func ErrorType(err error) string { return fmt.Sprintf("%T", err) }

// Error пишет неожиданный сбой один раз на границе, где его обрабатывают.
func Error(ctx context.Context, message string, err error, attrs ...any) {
	attrs = append(attrs, "error_type", ErrorType(err))
	slog.ErrorContext(ctx, message, attrs...)
}

// Fatal допустим только в main ПОСЛЕ возврата run: cleanup/defer в run уже выполнены.
// os.Exit(1) не вызывает оставшиеся defer. В Handler/UseCase/Repository Fatal запрещён.
func Fatal(message string, attrs ...any) {
	slog.Log(context.Background(), LevelFatal, message, attrs...)
	os.Exit(1)
}
