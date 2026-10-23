//go:build integration

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestS3PrivateUploadDownloadLifecycle(t *testing.T) {
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT is not set")
	}
	ctx := context.Background()
	st, err := New(ctx, Config{Endpoint: endpoint, Region: "us-east-1", Bucket: "chat-media-test", AccessKey: os.Getenv("TEST_S3_ACCESS_KEY"), SecretKey: os.Getenv("TEST_S3_SECRET_KEY"), PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	// This test owns a dedicated bucket and never touches the application bucket.
	_, err = st.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(st.bucket)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = st.client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(st.bucket)}) })
	key := fmt.Sprintf("staging/%d", time.Now().UnixNano())
	defer st.Delete(ctx, key)
	data := []byte("test image bytes")
	ticket, err := st.Upload(ctx, key, "image/png", int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	send := func(b []byte) int {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for k, v := range ticket.Fields {
			if err := writer.WriteField(k, v); err != nil {
				t.Fatal(err)
			}
		}
		part, err := writer.CreateFormFile("file", "test.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(b)
		_ = writer.Close()
		req, _ := http.NewRequestWithContext(ctx, "POST", ticket.URL, &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Origin", "http://localhost:5173")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode < 300 && response.Header.Get("Access-Control-Allow-Origin") == "" {
			t.Fatal("browser upload missing CORS")
		}
		return response.StatusCode
	}
	if status := send(data); status >= 300 {
		t.Fatalf("upload status %d", status)
	}
	if status := send(append(data, 'x')); status < 400 {
		t.Fatalf("size policy accepted extra bytes: %d", status)
	}
	read, err := st.Read(ctx, key, 100)
	if err != nil || !bytes.Equal(read, data) {
		t.Fatalf("read: %q %v", read, err)
	}
	response, err := http.Get(endpoint + "/" + st.bucket + "/" + key)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("anonymous access: %d", response.StatusCode)
	}
	url, err := st.Download(ctx, key, "image/png", true)
	if err != nil {
		t.Fatal(err)
	}
	response, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !bytes.Equal(b, data) || response.Header.Get("Content-Disposition") != `attachment; filename="image.png"` {
		t.Fatalf("download status=%d", response.StatusCode)
	}
	expired, err := st.signer.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(st.bucket), Key: aws.String(key)}, func(o *s3.PresignOptions) { o.Expires = time.Second })
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	response, err = http.Get(expired.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("expired URL accepted: %d", response.StatusCode)
	}
	if err = st.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Read(ctx, key, 100); err == nil {
		t.Fatal("deleted object remains readable")
	}
}
