// Package authctx хранит только request-scoped identity для HTTP transport.
// Приватный тип ключа исключает коллизии с ключами других пакетов.
package authctx

import "context"

type userIDKey struct{}

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func UserID(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(userIDKey{}).(int64)
	return value, ok && value > 0
}
