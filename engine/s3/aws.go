package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

var (
	s3Client   *s3.Client
	bucketName string
	logsDir    = "logs"
	mutex      sync.Mutex
)

type OrderBookSnapshot struct {
	Timestamp     string               `json:"timestamp"`
	MarketName    string               `json:"marketName"`
	OrderBook     types.YesNoOrderBook `json:"orderBook"`
	USDBalances   types.USDBalances    `json:"usdBalances"`
	StockBalances types.StockBalances  `json:"stockBalances"`
}

// InitS3Logger initializes the S3 client
func InitS3Logger() error {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found for S3 logger, using system environment variables")
	}

	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	awsRegion := os.Getenv("AWS_REGION")
	bucketName = os.Getenv("S3_BUCKET_NAME")

	if awsAccessKey == "" || awsSecretKey == "" || awsRegion == "" || bucketName == "" {
		return fmt.Errorf("AWS credentials or bucket name not configured")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsRegion),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	log.Println("S3 logger initialized successfully")
	return nil
}

// AppendOrderBookSnapshot appends the current order book snapshot to a local log file
func AppendOrderBookSnapshot(marketName string, orderBook types.YesNoOrderBook, usdBalances types.USDBalances, stockBalances types.StockBalances) error {
	mutex.Lock()
	defer mutex.Unlock()

	snapshot := OrderBookSnapshot{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		MarketName:    marketName,
		OrderBook:     orderBook,
		USDBalances:   usdBalances,
		StockBalances: stockBalances,
	}

	jsonData, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal order book snapshot: %v", err)
	}

	// Create filename based on market name and date
	filename := fmt.Sprintf("%s_%s.log", marketName, time.Now().UTC().Format("2006-01-02"))
	filepath := filepath.Join(logsDir, filename)

	// Append to file
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}

	return nil
}

// UploadLogsToS3 uploads all log files to S3
func UploadLogsToS3() error {
	mutex.Lock()
	defer mutex.Unlock()

	files, err := os.ReadDir(logsDir)
	if err != nil {
		return fmt.Errorf("failed to read logs directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		localPath := filepath.Join(logsDir, file.Name())
		s3Key := fmt.Sprintf("orderbook/%s/%s", file.Name(), time.Now().UTC().Format("2006-01-02_15-04-05"))

		if err := uploadFileToS3(localPath, s3Key); err != nil {
			log.Printf("Failed to upload %s to S3: %v", file.Name(), err)
			continue
		}

		log.Printf("Successfully uploaded %s to S3 as %s", file.Name(), s3Key)
	}

	return nil
}

// uploadFileToS3 uploads a single file to S3
func uploadFileToS3(localPath, s3Key string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", localPath, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	buffer := make([]byte, fileInfo.Size())
	_, err = file.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
		Body:   bytes.NewReader(buffer),
	})

	return err
}

// StartPeriodicUpload starts a goroutine that uploads logs to S3 every 10 seconds
func StartPeriodicUpload(orderBook types.YesNoOrderBook, usdBalances types.USDBalances, stockBalances types.StockBalances) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Append current snapshot to local files for all markets
			for marketName := range orderBook {
				if err := AppendOrderBookSnapshot(marketName, orderBook, usdBalances, stockBalances); err != nil {
					log.Printf("Failed to append order book snapshot for %s: %v", marketName, err)
				}
			}

			// Upload all log files to S3
			if err := UploadLogsToS3(); err != nil {
				log.Printf("Failed to upload logs to S3: %v", err)
			}
		}
	}()
}
