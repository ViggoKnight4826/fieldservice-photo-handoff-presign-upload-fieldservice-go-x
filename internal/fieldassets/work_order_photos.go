package fieldassets

import (
	"context"
	"fmt"
	"regexp"
)

const (
	StatusAwaitingPhoto    = "awaiting_photo"
	StatusReadyForDispatch = "ready_for_dispatch"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var safePhotoID = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type Storage interface {
	CreateBucket(context.Context, string) error
	PresignPhoto(context.Context, string, string, string, string, int64) (string, error)
	PhotoExists(context.Context, string, string) (bool, error)
}

type Service struct {
	storage Storage
	bucket  string
}

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func NewService(storage Storage, bucket string) *Service {
	return &Service{storage: storage, bucket: bucket}
}

type UploadRequest struct {
	WorkOrderID  string `json:"work_order_id"`
	TechnicianID string `json:"technician_id"`
	PhotoID      string `json:"photo_id"`
	ContentType  string `json:"content_type"`
	Bytes        int64  `json:"bytes"`
	RequestID    string `json:"request_id"`
}

type UploadTicket struct {
	UploadURL      string `json:"upload_url"`
	Method         string `json:"method"`
	ObjectKey      string `json:"object_key"`
	DispatchStatus string `json:"dispatch_status"`
}

type FollowUp struct {
	WorkOrderID      string `json:"work_order_id"`
	DispatchStatus   string `json:"dispatch_status"`
	TechnicianAction string `json:"technician_action"`
}

func (s *Service) Prepare(ctx context.Context) error {
	return s.storage.CreateBucket(ctx, s.bucket)
}

func (s *Service) IssueUpload(ctx context.Context, in UploadRequest) (UploadTicket, error) {
	if !safeID.MatchString(in.WorkOrderID) || !safeID.MatchString(in.TechnicianID) || !safePhotoID.MatchString(in.PhotoID) || in.RequestID == "" {
		return UploadTicket{}, &ValidationError{Message: "work_order_id, technician_id, photo_id, and request_id are required"}
	}
	if in.ContentType != "image/jpeg" && in.ContentType != "image/png" {
		return UploadTicket{}, &ValidationError{Message: "content_type must be image/jpeg or image/png"}
	}
	if in.Bytes <= 0 || in.Bytes > 10*1024*1024 {
		return UploadTicket{}, &ValidationError{Message: "bytes must be between 1 and 10485760"}
	}
	key := fmt.Sprintf("work-orders/%s/technicians/%s/%s", in.WorkOrderID, in.TechnicianID, in.PhotoID)
	uploadURL, err := s.storage.PresignPhoto(ctx, s.bucket, key, in.ContentType, in.RequestID, in.Bytes)
	if err != nil {
		return UploadTicket{}, err
	}
	return UploadTicket{UploadURL: uploadURL, Method: "PUT", ObjectKey: key, DispatchStatus: StatusAwaitingPhoto}, nil
}

func (s *Service) ConfirmUpload(ctx context.Context, workOrderID, technicianID, photoID string) (FollowUp, error) {
	if !safeID.MatchString(workOrderID) || !safeID.MatchString(technicianID) || !safePhotoID.MatchString(photoID) {
		return FollowUp{}, &ValidationError{Message: "invalid work order photo identity"}
	}
	key := fmt.Sprintf("work-orders/%s/technicians/%s/%s", workOrderID, technicianID, photoID)
	found, err := s.storage.PhotoExists(ctx, s.bucket, key)
	if err != nil {
		return FollowUp{}, err
	}
	if !found {
		return FollowUp{WorkOrderID: workOrderID, DispatchStatus: StatusAwaitingPhoto, TechnicianAction: "upload_photo"}, nil
	}
	return FollowUp{WorkOrderID: workOrderID, DispatchStatus: StatusReadyForDispatch, TechnicianAction: "review_dispatch_notes"}, nil
}
