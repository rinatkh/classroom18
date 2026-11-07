// Package auth содержит concrete implementations для password hashing и JWT.
// Пакет user зависит не от них, а от небольших интерфейсов потребителя.
package auth

import "golang.org/x/crypto/bcrypt"

type BcryptHasher struct {
	cost int
}

func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(password string) (string, error) {
	value, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	return string(value), err
}

func (h *BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
