package app

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"hash"
	"strings"
	"time"
)

const (
	EPOCH = 1293840000
)

type Signer struct {
	h   hash.Hash
	Sep string
}

func NewSigner(h hash.Hash) *Signer {
	return &Signer{
		h:   h,
		Sep: ".",
	}
}

func base64URLEncode(data []byte) string {
	var result = base64.URLEncoding.EncodeToString(data)
	return strings.TrimRight(result, "=")
}

func base64URLDecode(data string) ([]byte, error) {
	var missing = (4 - len(data)%4) % 4
	data += strings.Repeat("=", missing)
	return base64.URLEncoding.DecodeString(data)
}

func splitRight(b string, sep string) (left string, right string, ok bool) {
	idx := strings.LastIndex(b, sep)
	if idx <= -1 {
		return "", "", false
	}

	return b[:idx], b[idx+1:], true
}

func intToBytes(unixTime int64) []byte {
	b := make([]byte, 0, 8)
	for i := uint(0); unixTime > 0; i++ {
		unixTime >>= i * 8
		b = append(b, byte(unixTime))
	}
	return b
}

func bytesToInt(b []byte) int64 {
	var unixTime int64
	for i, v := range b {
		pos := len(b) - 1 - i
		unixTime |= int64(v) << (uint(pos) * 8)
	}
	unixTime += EPOCH
	return unixTime
}

func (s *Signer) signature(b string) string {
	s.h.Reset()
	s.h.Write([]byte(b))
	return base64URLEncode(s.h.Sum(nil))
}

func (s *Signer) Sign(msg string) string {
	return msg + s.Sep + s.signature(msg)
}

func (s Signer) Verify(b string) (string, error) {
	msg, signature, ok := splitRight(b, s.Sep)
	if !ok {
		return "", errors.New("Couldn't extract signature")
	}
	actual := s.signature(msg)
	if subtle.ConstantTimeCompare([]byte(signature), []byte(actual)) != 1 {
		return "", errors.New("Signature's don't match")
	}
	return msg, nil
}

type TimestampSigner struct {
	*Signer
}

func NewTimestampSigner(h hash.Hash) *TimestampSigner {
	return &TimestampSigner{
		Signer: NewSigner(h),
	}
}

func (s *TimestampSigner) Sign(msg string) string {
	return s.SignWithTime(msg, time.Now().Unix())
}

func (s *TimestampSigner) SignWithTime(msg string, now int64) string {
	return s.Signer.Sign(msg + s.Sep + base64URLEncode(intToBytes(now-EPOCH)))
}

func (s *TimestampSigner) Verify(b string, dur time.Duration) (string, error) {
	return s.VerifyWithTime(b, time.Now().Unix(), dur)
}

func (s *TimestampSigner) VerifyWithTime(b string, now int64, dur time.Duration) (string, error) {
	msg, err := s.Signer.Verify(b)
	if err != nil {
		return "", err
	}

	msg, timeB64, ok := splitRight(msg, s.Sep)
	if !ok {
		return "", errors.New("Failed to extract timestamp")
	}

	timeBytes, err := base64URLDecode(timeB64)
	if err != nil {
		return "", err
	}

	unixTime := bytesToInt(timeBytes) + EPOCH

	if time.Unix(now, 0).Sub(time.Unix(unixTime, 0)) > dur {
		return "", errors.New("Token expired")
	}

	return msg, nil
}
