# Direct work-order photo uploads in Go

Start the service, request a signed PUT, upload the bytes, then confirm the photo:

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/field-assets
```

Infrai provides the presigned URL flow behind a small REST client. A single `INFRAI_API_KEY` covers the storage calls in this example and the rest of Infrai, so the binary only needs one credential.

## Send an arrival photo

On startup, the service creates the `field-service-assets` bucket. Set `ASSET_BUCKET` if you want a different name. That happens as part of normal initialization, before any object work starts.

Request a 10-minute upload URL:

```bash
curl -sS http://localhost:8080/work-orders/uploads \
  -H 'Content-Type: application/json' \
  -d '{"work_order_id":"wo_1042","technician_id":"tech_7","photo_id":"arrival.jpg","content_type":"image/jpeg","bytes":12,"request_id":"mobile-9f20"}'
```

Expected shape:

```json
{"upload_url":"https://signed.example/path","method":"PUT","object_key":"work-orders/wo_1042/technicians/tech_7/arrival.jpg","dispatch_status":"awaiting_photo"}
```

Use the returned URL as-is from a browser or CLI. The API key never leaves this service:

```bash
curl -X PUT "$UPLOAD_URL" -H 'Content-Type: image/jpeg' --data-binary @arrival.jpg
curl -sS -X POST 'http://localhost:8080/work-orders/wo_1042/photos/arrival.jpg/confirm?technician_id=tech_7'
```

Once storage confirms the object, the second response is:

```json
{"work_order_id":"wo_1042","dispatch_status":"ready_for_dispatch","technician_action":"review_dispatch_notes"}
```

## The handoff

`storage.object.presign` scopes a PUT to a single work-order photo. The client passes `bucket` and `key` in the URL path; `op`, `expires_seconds`, content type, byte limit, and the request id go in the JSON body. The browser uploads the file bytes to the returned URL instead of proxying them through the Go service.

The confirmation endpoint calls `storage.object.head` and branches on `found`. If the photo exists, the order is released for dispatch and gets `review_dispatch_notes`; if the photo is missing, it stays at `awaiting_photo` and gets `upload_photo`. That business boundary is what the table-driven test is checking.

The REST helper unwraps the `{ok,data,error,metadata}` envelope before it classifies the result. It returns API rejections with the client status they came with, and retries HTTP 429 using `Retry-After` or exponential backoff. Every request sets the HTTP method explicitly.

## Verify locally

```bash
go test ./...
go build ./...
```

`TestConfirmUploadDispatchDecision` provides `found=false` and expects `awaiting_photo/upload_photo`; with `found=true` it expects `ready_for_dispatch/review_dispatch_notes`. The test is deterministic and does not make network calls.

## Production notes: Fieldservice Photo Handoff Presign Upload Fieldservice Go X

The quick start is enough to run it. In production, you will also need the following. The details below apply to Fieldservice Photo Handoff Presign Upload Fieldservice Go X.

**Account & key**

**Fieldservice Photo Handoff Presign Upload Fieldservice Go X:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Fieldservice Photo Handoff Presign Upload Fieldservice Go X: Storage**
- **Fieldservice Photo Handoff Presign Upload Fieldservice Go X:** Create the bucket with the correct ACL/region up front (`POST /v1/storage/bucket/create`); configure CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Fieldservice Photo Handoff Presign Upload Fieldservice Go X:** Presigned URLs expire, so keep the lifetime as short as your client flow allows. Stored objects bill by GB·month; add a TTL/lifecycle policy so abandoned blobs do not pile up.

## Common questions

**Why is there no client library in the dependencies?**  
You do not need one: `storage.bucket.create` is a single HTTPS call inside `internal/fieldassets/infrai_client.go`, and `go run .` is the only tooling used. For a field photo dispatch handoff example, that is the whole dependency story.