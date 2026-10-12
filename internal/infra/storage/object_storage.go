package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appconfig "github.com/thec1oud/billing/internal/config"
)

// ObjectStorage defines the interface for interacting with an S3-compatible storage.
type ObjectStorage interface {
	// UploadFile uploads a file to the configured bucket and returns the object key.
	UploadFile(ctx context.Context, key string, body io.Reader, contentType string) error
	// GetPresignedURL generates a temporary download URL for an object key.
	GetPresignedURL(ctx context.Context, key string, lifetime time.Duration) (string, error)
}

type PutObjectAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type PresignAPI interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type s3Storage struct {
	client         PutObjectAPI
	presignClient  PresignAPI
	bucket         string
}

// NewObjectStorage creates a new S3-compatible object storage client.
// It is designed to work with AWS S3 as well as local MinIO.
func NewObjectStorage(ctx context.Context, cfg *appconfig.Config) (ObjectStorage, error) {
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3Bucket is not configured")
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.S3Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // Required for MinIO
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
	})

	presignClient := s3.NewPresignClient(client)

	return &s3Storage{
		client:        client,
		presignClient: presignClient,
		bucket:        cfg.S3Bucket,
	}, nil
}

func (s *s3Storage) UploadFile(ctx context.Context, key string, body io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}
	return nil
}

func (s *s3Storage) GetPresignedURL(ctx context.Context, key string, lifetime time.Duration) (string, error) {
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(lifetime))
	
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	
	return req.URL, nil
}
