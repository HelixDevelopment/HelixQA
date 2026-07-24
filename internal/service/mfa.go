package service

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"

	"github.com/pquerna/otp/totp"
)

type MFAService struct{}

func NewMFAService() *MFAService {
	return &MFAService{}
}

func (s *MFAService) GenerateSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base32.StdEncoding.EncodeToString(buf), nil
}

func (s *MFAService) GenerateRecoveryCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		buf := make([]byte, 10)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("generate recovery code: %w", err)
		}
		codes[i] = base32.StdEncoding.EncodeToString(buf)[:16]
	}
	return codes, nil
}

func (s *MFAService) Verify(secret, code string) bool {
	return totp.Validate(code, secret)
}

func (s *MFAService) TotpURL(issuer, email, secret string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", issuer, email, secret, issuer)
}
