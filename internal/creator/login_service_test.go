package creator

import (
	"context"
	"errors"
	"testing"
)

type fakeOTP struct {
	verifyErr error
	sentTo    string
	checkedTo string
}

func (f *fakeOTP) SendOTP(_ context.Context, to, _ string) error {
	f.sentTo = to
	return nil
}

func (f *fakeOTP) VerifyOTP(_ context.Context, to, _, _ string) error {
	f.checkedTo = to
	return f.verifyErr
}

func TestCompleteControlsCreatorFulfillment(t *testing.T) {
	tests := []struct {
		name       string
		verifyErr  error
		wantAccess bool
	}{
		{name: "verified phone releases purchase", wantAccess: true},
		{name: "rejected code keeps purchase closed", verifyErr: errors.New("code rejected")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &fakeOTP{verifyErr: tt.verifyErr}
			service := NewLoginService(gateway)
			service.newID = func() (string, error) { return "challenge-42", nil }

			id, err := service.Start(context.Background(), "+15551234567", "creator-7", "asset-9")
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			result, err := service.Complete(context.Background(), id, "123456")
			if (err == nil) != tt.wantAccess {
				t.Fatalf("Complete() error = %v, want access %v", err, tt.wantAccess)
			}
			if gateway.sentTo != gateway.checkedTo {
				t.Fatalf("handoff phone = %q, sent phone = %q", gateway.checkedTo, gateway.sentTo)
			}
			if tt.wantAccess {
				if result.DeliveryStatus != "released" || result.SubscriberStatus != "active" || result.ContentStatus != "queued" {
					t.Fatalf("unexpected fulfillment: %+v", result)
				}
				if _, err := service.Complete(context.Background(), id, "123456"); !errors.Is(err, ErrChallengeUsed) {
					t.Fatalf("second Complete() error = %v", err)
				}
			}
		})
	}
}
