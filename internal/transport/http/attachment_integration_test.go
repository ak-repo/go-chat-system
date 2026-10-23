//go:build integration

package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/platform/storage"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/service"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/ak-repo/go-chat-system/internal/transport/wrapper"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run only against a disposable, migrated database and disposable MinIO.
func TestImageHTTPLifecycle(t *testing.T) {
	dsn, endpoint := os.Getenv("TEST_MEDIA_DATABASE_URL"), os.Getenv("TEST_S3_ENDPOINT")
	if dsn == "" || endpoint == "" {
		t.Skip("TEST_MEDIA_DATABASE_URL and TEST_S3_ENDPOINT required")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bucket := "chat-media-test-" + uuid.NewString()
	cfg := storage.Config{Endpoint: endpoint, Region: "us-east-1", Bucket: bucket, AccessKey: os.Getenv("TEST_S3_ACCESS_KEY"), SecretKey: os.Getenv("TEST_S3_SECRET_KEY"), PathStyle: true}
	client := s3.NewFromConfig(aws.Config{Region: cfg.Region, Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")}, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true })
	if _, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	defer client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	store, err := storage.New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	users := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	for _, id := range users {
		_, err = db.Exec(ctx, `INSERT INTO users(id,username,email,password_hash,role) VALUES($1,$2,$3,'test-only','user')`, id, "u"+id[:8], id+"@example.test")
		if err != nil {
			t.Fatal(err)
		}
	}
	friends := repository.NewFriendRepositoryImpl(db)
	if err = friends.CreateFriendship(ctx, users[0], users[1]); err != nil {
		t.Fatal(err)
	}
	if err = friends.CreateFriendship(ctx, users[1], users[2]); err != nil {
		t.Fatal(err)
	}
	conversations := repository.NewConversationRepository(db)
	first, err := conversations.CreateOrGetDirect(ctx, users[0], users[1])
	if err != nil {
		t.Fatal(err)
	}
	second, err := conversations.CreateOrGetDirect(ctx, users[1], users[2])
	if err != nil {
		t.Fatal(err)
	}
	messages := service.NewMessageServiceImpl(repository.NewMessageRepositoryImpl(db), friends, repository.BlockRepositoryInit(db))
	messages.SetConversationRepository(conversations)
	attachments := repository.NewAttachmentRepository(db)
	media := service.NewAttachmentService(attachments, store, messages)
	handler := &AttachmentHandler{Service: media}
	workerCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); media.Run(workerCtx) }()
	defer func() { stop(); <-done }()
	router := chi.NewRouter()
	// Inject authenticated identities at the boundary. JWT/session middleware has
	// separate tests; no production route accepts this test-only header.
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, r.Header.Get("Test-Actor"))))
		})
	})
	router.Post("/conversations/{conversationID}/attachments", wrapper.HTTPResponseWrapper(handler.Initiate))
	router.Post("/attachments/{attachmentID}/complete", wrapper.HTTPResponseWrapper(handler.Complete))
	router.Get("/attachments/{attachmentID}", wrapper.HTTPResponseWrapper(handler.Status))
	router.Get("/messages/{messageID}/attachments/{attachmentID}/url", wrapper.HTTPResponseWrapper(handler.URL))
	router.Post("/conversations/{conversationID}/messages", wrapper.HTTPResponseWrapper(messages.Send))
	router.Get("/conversations/{conversationID}/messages", wrapper.HTTPResponseWrapper(messages.History))
	router.Post("/conversations/{conversationID}/messages/{messageID}/forward", wrapper.HTTPResponseWrapper(messages.Forward))
	router.Patch("/messages/{messageID}", wrapper.HTTPResponseWrapper(messages.Edit))
	router.Delete("/messages/{messageID}", wrapper.HTTPResponseWrapper(messages.Delete))
	server := httptest.NewServer(router)
	defer server.Close()
	request := func(method, path, actor string, body any, want int) json.RawMessage {
		t.Helper()
		encoded, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(encoded))
		req.Header.Set("Test-Actor", actor)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		b, _ := io.ReadAll(response.Body)
		if response.StatusCode != want {
			t.Fatalf("%s %s status=%d want=%d", method, path, response.StatusCode, want)
		}
		if err = json.Unmarshal(b, &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}
	var imageBytes bytes.Buffer
	_ = png.Encode(&imageBytes, image.NewNRGBA(image.Rect(0, 0, 4, 2)))
	raw := request("POST", "/conversations/"+first.ID+"/attachments", users[0], map[string]any{"filename": "photo.png", "mime_type": "image/png", "size_bytes": imageBytes.Len()}, 201)
	var ticket struct {
		Attachment model.Attachment `json:"attachment"`
		Upload     storage.Upload   `json:"upload"`
	}
	if err = json.Unmarshal(raw, &ticket); err != nil {
		t.Fatal(err)
	}
	saved, err := attachments.Get(ctx, ticket.Attachment.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, key := range []string{saved.StagingKey, saved.FullKey, saved.ThumbnailKey} {
			_ = store.Delete(ctx, key)
		}
	}()
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	for k, v := range ticket.Upload.Fields {
		_ = writer.WriteField(k, v)
	}
	part, _ := writer.CreateFormFile("file", "photo.png")
	_, _ = part.Write(imageBytes.Bytes())
	_ = writer.Close()
	response, err := http.Post(ticket.Upload.URL, writer.FormDataContentType(), &upload)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode >= 300 {
		t.Fatalf("storage upload=%d", response.StatusCode)
	}
	request("GET", "/attachments/"+saved.ID, users[1], nil, 404)
	request("POST", "/attachments/"+saved.ID+"/complete", users[0], nil, 202)
	deadline := time.Now().Add(15 * time.Second)
	for {
		a, err := attachments.Get(ctx, saved.ID)
		if err != nil {
			t.Fatal(err)
		}
		if a.State == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("processing state=%s", a.State)
		}
		time.Sleep(100 * time.Millisecond)
	}
	payload := map[string]any{"content": "caption", "attachment_ids": []string{saved.ID}, "client_message_id": uuid.NewString(), "sender_id": users[2]}
	// Two concurrent retries must return the same committed message.
	var results [2]json.RawMessage
	var group sync.WaitGroup
	for i := range results {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			results[i] = request("POST", "/conversations/"+first.ID+"/messages", users[0], payload, 201)
		}(i)
	}
	group.Wait()
	var source, retry model.Message
	_ = json.Unmarshal(results[0], &source)
	_ = json.Unmarshal(results[1], &retry)
	if source.ID != retry.ID || source.SenderID != users[0] || len(source.Attachments) != 1 {
		t.Fatalf("deduplication/identity failed")
	}
	request("PATCH", "/messages/"+source.ID, users[0], map[string]string{"content": ""}, 200)
	history := request("GET", "/conversations/"+first.ID+"/messages", users[1], nil, 200)
	if !bytes.Contains(history, []byte(`"message_type":"image"`)) {
		t.Fatal("history lost image metadata")
	}
	path := fmt.Sprintf("/messages/%s/attachments/%s/url", source.ID, saved.ID)
	request("GET", path, users[2], nil, 404)
	raw = request("GET", path, users[1], nil, 200)
	var link struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(raw, &link)
	response, err = http.Get(link.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("recipient download failed")
	}
	raw = request("POST", "/conversations/"+second.ID+"/messages/"+source.ID+"/forward", users[1], map[string]string{"client_message_id": uuid.NewString()}, 201)
	var forward model.Message
	_ = json.Unmarshal(raw, &forward)
	request("DELETE", "/messages/"+source.ID, users[0], nil, 200)
	request("GET", path, users[1], nil, 404)
	request("GET", fmt.Sprintf("/messages/%s/attachments/%s/url", forward.ID, saved.ID), users[2], nil, 200)
	// Expire the test policy so worker cleanup can run immediately.
	if _, err = db.Exec(ctx, `UPDATE attachments SET upload_expires_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	request("DELETE", "/messages/"+forward.ID, users[1], nil, 200)
	deadline = time.Now().Add(15 * time.Second)
	for {
		a, err := attachments.Get(ctx, saved.ID)
		if err != nil {
			t.Fatal(err)
		}
		if a.State == "deleted" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cleanup state=%s", a.State)
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, key := range []string{saved.StagingKey, saved.FullKey, saved.ThumbnailKey} {
		if _, err = store.Read(ctx, key, 100000); err == nil {
			t.Fatalf("object survives cleanup")
		}
	}
}
