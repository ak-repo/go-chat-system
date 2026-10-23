package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Upload struct {
	URL    string            `json:"url"`
	Fields map[string]string `json:"fields"`
}

type Store interface {
	Upload(context.Context, string, string, int64) (*Upload, error)
	Read(context.Context, string, int64) ([]byte, error)
	Put(context.Context, string, string, []byte) error
	Delete(context.Context, string) error
	Download(context.Context, string, string, bool) (string, error)
}

type Config struct {
	Endpoint, Region, Bucket, AccessKey, SecretKey string
	PathStyle                                      bool
}
type S3 struct {
	client *s3.Client
	signer *s3.PresignClient
	bucket string
}

func New(ctx context.Context, c Config) (*S3, error) {
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(c.Region), awsconfig.WithHTTPClient(&http.Client{Timeout: 20 * time.Second})}
	if c.AccessKey != "" {
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, "")))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = c.PathStyle
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
	})
	return &S3{client: client, signer: s3.NewPresignClient(client), bucket: c.Bucket}, nil
}
func (s *S3) Upload(ctx context.Context, key, mime string, size int64) (*Upload, error) {
	p, err := s.signer.PresignPostObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(mime)}, func(o *s3.PresignPostOptions) {
		o.Expires = 10 * time.Minute
		o.Conditions = []any{[]any{"content-length-range", size, size}, map[string]string{"Content-Type": mime}}
	})
	if err != nil {
		return nil, err
	}
	p.Values["Content-Type"] = mime
	return &Upload{URL: p.URL, Fields: p.Values}, nil
}
func (s *S3) Read(ctx context.Context, key string, limit int64) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer obj.Body.Close()
	if obj.ContentLength != nil && *obj.ContentLength > limit {
		return nil, fmt.Errorf("object exceeds size limit")
	}
	b, err := io.ReadAll(io.LimitReader(obj.Body, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("object exceeds size limit")
	}
	return b, err
}
func (s *S3) Put(ctx context.Context, key, mime string, b []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(mime), Body: bytes.NewReader(b)})
	return err
}
func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}
func (s *S3) Download(ctx context.Context, key, mime string, download bool) (string, error) {
	disposition := "inline"
	if download {
		disposition = `attachment; filename="image.jpg"`
		if mime == "image/png" {
			disposition = `attachment; filename="image.png"`
		}
	}
	p, err := s.signer.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ResponseContentType: aws.String(mime), ResponseContentDisposition: aws.String(disposition), ResponseCacheControl: aws.String("private, max-age=240")}, func(o *s3.PresignOptions) { o.Expires = 5 * time.Minute })
	if err != nil {
		return "", err
	}
	return p.URL, nil
}
