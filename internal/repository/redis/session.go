// Package redis — adapter прежнего user.RefreshSessionRepository.
package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rinatkh/classroom18/internal/user"
)

type RefreshSessionRepository struct {
	client *goredis.Client
	prefix string
}

var _ user.RefreshSessionRepository = (*RefreshSessionRepository)(nil)

func NewRefreshSessionRepository(client *goredis.Client) *RefreshSessionRepository {
	return &RefreshSessionRepository{client: client, prefix: "auth:{refresh}:"}
}

// Hash поля — строки: int64 user ID не проходит через float64/JSON Lua.
// Скрипты выполняют проверку и изменение без interleaving других клиентов.
const createScript = `
local now = redis.call('TIME')
local ms = tonumber(now[1])*1000 + math.floor(tonumber(now[2])/1000)
if tonumber(ARGV[2]) <= ms then return -2 end
if redis.call('EXISTS',KEYS[1]) == 1 then return -1 end
redis.call('INCR',KEYS[2])
local id = redis.call('GET',KEYS[2])
redis.call('HSET',KEYS[1],'id',tostring(id),'user_id',ARGV[1],
 'expires_at',ARGV[2],'created_at',ARGV[3])
redis.call('PEXPIREAT',KEYS[1],ARGV[2])
return id`

func (r *RefreshSessionRepository) Create(ctx context.Context, s user.RefreshSession) (user.RefreshSession, error) {
	if s.UserID <= 0 || s.TokenHash == "" || !validExpiration(s.ExpiresAt) {
		return user.RefreshSession{}, user.ErrInvalidRefreshToken
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	id, err := r.client.Eval(ctx, createScript, []string{r.key(s.TokenHash), r.prefix + "sequence"},
		s.UserID, s.ExpiresAt.UnixMilli(), s.CreatedAt.UnixMilli()).Int64()
	if err != nil {
		return user.RefreshSession{}, fmt.Errorf("redis create session: %w", err)
	}
	if id == -1 {
		return user.RefreshSession{}, user.ErrSessionAlreadyExists
	}
	if id == -2 {
		return user.RefreshSession{}, user.ErrRefreshExpired
	}
	s.ID = id
	s.CreatedAt = s.CreatedAt.Truncate(time.Millisecond)
	s.ExpiresAt = s.ExpiresAt.Truncate(time.Millisecond)
	return s, nil
}

func (r *RefreshSessionRepository) GetByTokenHash(ctx context.Context, hash string) (user.RefreshSession, error) {
	fields, err := r.client.HGetAll(ctx, r.key(hash)).Result()
	if err != nil {
		return user.RefreshSession{}, fmt.Errorf("redis get session: %w", err)
	}
	if len(fields) == 0 {
		return user.RefreshSession{}, user.ErrInvalidRefreshToken
	}
	s := user.RefreshSession{TokenHash: hash}
	var value int64
	for name, dest := range map[string]*int64{"id": &s.ID, "user_id": &s.UserID} {
		*dest, err = strconv.ParseInt(fields[name], 10, 64)
		if err != nil {
			return user.RefreshSession{}, fmt.Errorf("corrupt session %s", name)
		}
	}
	for name, dest := range map[string]*time.Time{"created_at": &s.CreatedAt, "expires_at": &s.ExpiresAt} {
		value, err = strconv.ParseInt(fields[name], 10, 64)
		if err != nil {
			return user.RefreshSession{}, fmt.Errorf("corrupt session %s", name)
		}
		*dest = time.UnixMilli(value).UTC()
	}
	if fields["revoked_at"] != "" {
		value, err = strconv.ParseInt(fields["revoked_at"], 10, 64)
		if err != nil {
			return user.RefreshSession{}, fmt.Errorf("corrupt revoked_at")
		}
		at := time.UnixMilli(value).UTC()
		s.RevokedAt = &at
	}
	if fields["replaced_by"] != "" {
		value, err = strconv.ParseInt(fields["replaced_by"], 10, 64)
		if err != nil {
			return user.RefreshSession{}, fmt.Errorf("corrupt replaced_by")
		}
		s.ReplacedBySessionID = &value
	}
	return s, nil
}

const rotateScript = `
if redis.call('EXISTS',KEYS[1]) == 0 then return 1 end
if redis.call('HEXISTS',KEYS[1],'revoked_at') == 1 then return 2 end
local now = redis.call('TIME')
local ms = tonumber(now[1])*1000 + math.floor(tonumber(now[2])/1000)
if tonumber(redis.call('HGET',KEYS[1],'expires_at')) <= ms then return 3 end
if redis.call('HGET',KEYS[1],'user_id') ~= ARGV[1] then return 1 end
if tonumber(ARGV[2]) <= ms then return 3 end
if KEYS[1] == KEYS[2] or redis.call('EXISTS',KEYS[2]) == 1 then return 4 end
redis.call('INCR',KEYS[3])
local id = redis.call('GET',KEYS[3])
redis.call('HSET',KEYS[2],'id',tostring(id),'user_id',ARGV[1],
 'expires_at',ARGV[2],'created_at',ARGV[3])
redis.call('PEXPIREAT',KEYS[2],ARGV[2])
redis.call('HSET',KEYS[1],'revoked_at',tostring(ms),'replaced_by',tostring(id))
return 0`

func (r *RefreshSessionRepository) Rotate(ctx context.Context, oldHash string, next user.RefreshSession) error {
	if oldHash == "" || next.TokenHash == "" || next.UserID <= 0 || !validExpiration(next.ExpiresAt) {
		return user.ErrInvalidRefreshToken
	}
	if next.CreatedAt.IsZero() {
		next.CreatedAt = time.Now().UTC()
	}
	code, err := r.client.Eval(ctx, rotateScript, []string{r.key(oldHash), r.key(next.TokenHash), r.prefix + "sequence"},
		next.UserID, next.ExpiresAt.UnixMilli(), next.CreatedAt.UnixMilli()).Int()
	if err != nil {
		return fmt.Errorf("redis rotate session: %w", err)
	}
	switch code {
	case 0:
		return nil
	case 1:
		return user.ErrInvalidRefreshToken
	case 2:
		return user.ErrRefreshRevoked
	case 3:
		return user.ErrRefreshExpired
	case 4:
		return user.ErrSessionAlreadyExists
	default:
		return fmt.Errorf("unexpected rotation result")
	}
}

const revokeScript = `
if redis.call('EXISTS',KEYS[1]) == 0 then return 0 end
if redis.call('HEXISTS',KEYS[1],'revoked_at') == 0 then
 local now=redis.call('TIME')
 local ms=tonumber(now[1])*1000 + math.floor(tonumber(now[2])/1000)
 redis.call('HSET',KEYS[1],'revoked_at',tostring(ms))
end
return 0`

func (r *RefreshSessionRepository) Revoke(ctx context.Context, hash string) error {
	if err := r.client.Eval(ctx, revokeScript, []string{r.key(hash)}).Err(); err != nil {
		return fmt.Errorf("redis revoke session: %w", err)
	}
	return nil
}
func (r *RefreshSessionRepository) key(hash string) string { return r.prefix + hash }

// Lua использует double: миллисекунды должны представляться точно.
// Проверка выполняется до любых изменений сессии, а не после HSET.
func validExpiration(value time.Time) bool {
	return value.Year() >= 1 && value.Year() <= 9999 && value.UnixMilli() <= 9007199254740991
}
