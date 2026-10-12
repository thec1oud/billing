package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appconfig "github.com/thec1oud/billing/internal/config"
)

type mockPutObjectAPI struct {
	err error
}

func (m *mockPutObjectAPI) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &s3.PutObjectOutput{}, nil
}

type mockPresignAPI struct {
	err error
	url string
}

func (m *mockPresignAPI) PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &v4.PresignedHTTPRequest{URL: m.url}, nil
}

func TestNewObjectStorage_MissingBucket(t *testing.T) {
	cfg := &appconfig.Config{
		S3Bucket: "",
	}

	_, err := NewObjectStorage(context.Background(), cfg)
	if err == nil {
		t.Error("expected error due to missing S3Bucket, got nil")
	}
}

func TestNewObjectStorage_Success(t *testing.T) {
	cfg := &appconfig.Config{
		S3Bucket:    "test-bucket",
		S3Region:    "us-east-1",
		S3AccessKey: "test",
		S3SecretKey: "test",
		S3Endpoint:  "http://localhost:9000",
	}

	os, err := NewObjectStorage(context.Background(), cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if os == nil {
		t.Error("expected non-nil ObjectStorage")
	}
}

func TestUploadFile(t *testing.T) {
	mockClient := &mockPutObjectAPI{}
	storage := &s3Storage{
		client: mockClient,
		bucket: "test-bucket",
	}

	body := strings.NewReader("test content")
	err := storage.UploadFile(context.Background(), "test-key", body, "text/plain")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Test error case
	mockClient.err = errors.New("upload failed")
	err = storage.UploadFile(context.Background(), "test-key", body, "text/plain")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetPresignedURL(t *testing.T) {
	mockClient := &mockPresignAPI{
		url: "https://example.com/presigned",
	}
	storage := &s3Storage{
		presignClient: mockClient,
		bucket:        "test-bucket",
	}

	url, err := storage.GetPresignedURL(context.Background(), "test-key", time.Hour)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if url != "https://example.com/presigned" {
		t.Errorf("expected url https://example.com/presigned, got %s", url)
	}

	// Test error case
	mockClient.err = errors.New("presign failed")
	_, err = storage.GetPresignedURL(context.Background(), "test-key", time.Hour)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
