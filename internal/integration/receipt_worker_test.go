package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	appconfig "github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/infra/storage"
	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/receipt/model"
	"github.com/thec1oud/billing/internal/receipt/service"
	"github.com/thec1oud/billing/internal/receipt/worker"
	"github.com/thec1oud/billing/internal/shared/money"
)

// Mock InvoiceFetcher
type mockInvoiceFetcher struct{}

func (m *mockInvoiceFetcher) Get(ctx context.Context, invoiceID int64) (invoicemodel.Invoice, error) {
	now := time.Now()
	return invoicemodel.Invoice{
		InvoiceID:     invoiceID,
		AccountID:     202,
		InvoiceNumber: "INV-12345",
		Status:        invoicemodel.StatusPaid,
		Currency:      money.DefaultCurrency,
		LineItems: []invoicemodel.LineItem{
			{
				Description:   "Integration Test Item",
				QuantityValue: 1,
				QuantityUnit:  "pkg",
				UnitAmount:    money.MustNew(1000, money.DefaultCurrency),
				TotalAmount:   money.MustNew(1000, money.DefaultCurrency),
			},
		},
		AmountPaid: money.MustNew(1000, money.DefaultCurrency),
		PaidAt:     &now,
	}, nil
}

func createS3Bucket(ctx context.Context, endpoint, accessKey, secretKey, bucket string) error {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func TestReceiptWorker_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Setup Mock Fetcher
	fetcher := &mockInvoiceFetcher{}
	invoiceID := int64(123)

	// 3. Setup MinIO
	t.Log("Starting MinIO container...")
	// Ensure bin directory exists for MinIO data mount
	_ = os.MkdirAll("../../bin/minio_data", 0755)
	
	minioContainer, err := minio.Run(ctx, "quay.io/minio/minio:latest",
		minio.WithUsername("testuser"),
		minio.WithPassword("testpass123"),
	)
	if err != nil {
		t.Fatalf("failed to start minio: %v", err)
	}
	defer minioContainer.Terminate(context.Background())

	minioHost, err := minioContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get minio endpoint: %v", err)
	}
	minioUrl := "http://" + minioHost

	err = createS3Bucket(ctx, minioUrl, "testuser", "testpass123", "test-bucket")
	if err != nil {
		t.Fatalf("failed to create test-bucket: %v", err)
	}

	// 4. Setup RabbitMQ
	t.Log("Starting RabbitMQ container...")
	rmqContainer, err := rabbitmq.Run(ctx, "rabbitmq:3-management-alpine")
	if err != nil {
		t.Fatalf("failed to start rabbitmq: %v", err)
	}
	defer rmqContainer.Terminate(context.Background())

	rmqURL, err := rmqContainer.AmqpURL(ctx)
	if err != nil {
		t.Fatalf("failed to get amqp url: %v", err)
	}
	amqpConn, err := amqp091.Dial(rmqURL)
	if err != nil {
		t.Fatalf("failed to dial rabbitmq: %v", err)
	}
	defer amqpConn.Close()

	broker, err := messaging.NewRabbitBroker(amqpConn)
	if err != nil {
		t.Fatalf("failed to create broker: %v", err)
	}
	err = broker.InitTopology(ctx)
	if err != nil {
		t.Fatalf("failed to init broker topology: %v", err)
	}

	// 5. Setup Webhook Receiver
	t.Log("Starting Webhook Receiver...")
	webhookCh := make(chan model.ReceiptPayload, 1)
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload model.ReceiptPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode webhook payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		webhookCh <- payload
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	// 6. Initialize Worker Dependencies
	cfg := &appconfig.Config{
		ReceiptWebhookURL: webhookServer.URL,
		S3Endpoint:        minioUrl,
		S3Region:          "us-east-1",
		S3Bucket:          "test-bucket",
		S3AccessKey:       "testuser",
		S3SecretKey:       "testpass123",
	}

	deliverySvc := service.NewDeliveryService(cfg)
	generatorSvc := service.NewGeneratorService()
	objStorage, err := storage.NewObjectStorage(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to initialize object storage: %v", err)
	}

	consumer := worker.NewConsumer(broker, fetcher, generatorSvc, deliverySvc, objStorage)

	// Start consumer in background
	go func() {
		if err := consumer.Start(ctx); err != nil {
			t.Errorf("consumer start error: %v", err)
		}
	}()
	// Wait a bit for the consumer to register with RabbitMQ
	time.Sleep(1 * time.Second)

	// 7. Publish Event
	t.Log("Publishing invoice.paid event...")
	eventPayload := map[string]interface{}{
		"invoice_id": float64(invoiceID),
	}
	eventBytes, _ := json.Marshal(eventPayload)

	err = broker.PublishEvent(ctx, "invoice.paid", eventBytes)
	if err != nil {
		t.Fatalf("failed to publish event: %v", err)
	}

	// Wait for the webhook to be delivered
	t.Log("Waiting for webhook delivery...")
	select {
	case <-ctx.Done():
		t.Fatalf("webhook not received within timeout")
	case payload := <-webhookCh:
		if payload.DownloadURL == "" {
			t.Errorf("expected DownloadURL to be populated")
		}
		t.Logf("Webhook received successfully! Download URL: %s", payload.DownloadURL)

		// Download the PDF from MinIO using the presigned URL
		t.Log("Downloading PDF from S3 presigned URL...")
		resp, err := http.Get(payload.DownloadURL)
		if err != nil {
			t.Fatalf("failed to download PDF: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected HTTP 200 OK from presigned URL, got %d", resp.StatusCode)
		}

		pdfBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read PDF body: %v", err)
		}

		// Save the PDF locally in the bin/minio_data directory
		localPath := "../../bin/minio_data/integration_test_receipt.pdf"
		_ = os.MkdirAll("../../bin/minio_data", 0755)
		err = os.WriteFile(localPath, pdfBytes, 0644)
		if err != nil {
			t.Fatalf("failed to save PDF locally: %v", err)
		}
		t.Logf("Successfully saved generated receipt to: %s", localPath)
	case <-time.After(time.Second * 15):
		t.Fatal("timed out waiting for webhook delivery (integration test failed)")
	}
}
