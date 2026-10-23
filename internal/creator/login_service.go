package creator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrChallengeMissing = errors.New("login challenge not found")
	ErrChallengeUsed    = errors.New("login challenge already used")
)

type OTPGateway interface {
	SendOTP(context.Context, string, string) error
	VerifyOTP(context.Context, string, string, string) error
}

type Fulfillment struct {
	CreatorID        string `json:"creator_id"`
	AssetID          string `json:"asset_id"`
	DeliveryStatus   string `json:"delivery_status"`
	SubscriberStatus string `json:"subscriber_status"`
	ContentStatus    string `json:"content_status"`
}

type challenge struct {
	Phone     string
	CreatorID string
	AssetID   string
	Used      bool
	ExpiresAt time.Time
}

type LoginService struct {
	otp        OTPGateway
	now        func() time.Time
	newID      func() (string, error)
	mu         sync.Mutex
	challenges map[string]*challenge
}

func NewLoginService(otp OTPGateway) *LoginService {
	return &LoginService{
		otp:        otp,
		now:        time.Now,
		newID:      randomID,
		challenges: make(map[string]*challenge),
	}
}

func (s *LoginService) Start(ctx context.Context, phone, creatorID, assetID string) (string, error) {
	id, err := s.newID()
	if err != nil {
		return "", err
	}
	if err := s.otp.SendOTP(ctx, phone, id); err != nil {
		return "", err
	}

	s.mu.Lock()
	s.challenges[id] = &challenge{
		Phone: phone, CreatorID: creatorID, AssetID: assetID,
		ExpiresAt: s.now().Add(10 * time.Minute),
	}
	s.mu.Unlock()
	return id, nil
}

func (s *LoginService) Complete(ctx context.Context, challengeID, code string) (Fulfillment, error) {
	s.mu.Lock()
	entry, ok := s.challenges[challengeID]
	if !ok || s.now().After(entry.ExpiresAt) {
		s.mu.Unlock()
		return Fulfillment{}, ErrChallengeMissing
	}
	if entry.Used {
		s.mu.Unlock()
		return Fulfillment{}, ErrChallengeUsed
	}
	phone := entry.Phone
	s.mu.Unlock()

	if err := s.otp.VerifyOTP(ctx, phone, code, challengeID); err != nil {
		return Fulfillment{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if entry.Used {
		return Fulfillment{}, ErrChallengeUsed
	}
	entry.Used = true
	return Fulfillment{
		CreatorID:        entry.CreatorID,
		AssetID:          entry.AssetID,
		DeliveryStatus:   "released",
		SubscriberStatus: "active",
		ContentStatus:    "queued",
	}, nil
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
