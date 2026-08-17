package fieldassets

import (
	"context"
	"testing"
)

type fakeStorage struct{ found bool }

func (f fakeStorage) CreateBucket(context.Context, string) error { return nil }
func (f fakeStorage) PresignPhoto(context.Context, string, string, string, string, int64) (string, error) {
	return "https://upload.example/signed", nil
}
func (f fakeStorage) PhotoExists(context.Context, string, string) (bool, error) { return f.found, nil }

func TestConfirmUploadDispatchDecision(t *testing.T) {
	tests := []struct {
		name       string
		found      bool
		wantStatus string
		wantAction string
	}{
		{name: "photo is still pending", found: false, wantStatus: StatusAwaitingPhoto, wantAction: "upload_photo"},
		{name: "photo releases dispatch", found: true, wantStatus: StatusReadyForDispatch, wantAction: "review_dispatch_notes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(fakeStorage{found: tt.found}, "field-assets")
			got, err := service.ConfirmUpload(context.Background(), "wo_1042", "tech_7", "arrival.jpg")
			if err != nil {
				t.Fatal(err)
			}
			if got.DispatchStatus != tt.wantStatus || got.TechnicianAction != tt.wantAction {
				t.Fatalf("got status=%q action=%q, want status=%q action=%q", got.DispatchStatus, got.TechnicianAction, tt.wantStatus, tt.wantAction)
			}
		})
	}
}
