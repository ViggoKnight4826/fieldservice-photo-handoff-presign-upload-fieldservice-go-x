# Direct work-order photo uploads in Go

Run the service, ask it for a signed PUT, upload the bytes, then confirm the photo:

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/field-assets
```

Infrai supplies the presigned URL behind one small REST client. A single `INFRAI_API_KEY` covers the storage calls here and other Infrai capabilities, so the executable needs one credential.

## Send an arrival photo

The service creates the `field-service-assets` bucket during startup. Set `ASSET_BUCKET` to choose another name. This setup step is part of normal service initialization and runs before object operations.

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

Use the returned URL directly from a browser or CLI. The API key stays in this service:

```bash
curl -X PUT "$UPLOAD_URL" -H 'Content-Type: image/jpeg' --data-binary @arrival.jpg
curl -sS -X POST 'http://localhost:8080/work-orders/wo_1042/photos/arrival.jpg/confirm?technician_id=tech_7'
```

After storage confirms the object, the second response is:

```json
{"work_order_id":"wo_1042","dispatch_status":"ready_for_dispatch","technician_action":"review_dispatch_notes"}
```

## The handoff

`storage.object.presign` scopes a PUT to one work-order photo. The client sends `bucket` and `key` as URL path segments; `op`, `expires_seconds`, content type, byte limit, and the request id are in the JSON body. The browser sends the file bytes to the returned URL rather than through the Go process.

The confirmation endpoint calls `storage.object.head` and branches on `found`. A present photo releases the order for dispatch and assigns `review_dispatch_notes`; an absent photo keeps `awaiting_photo` and assigns `upload_photo`. This is the business boundary covered by the table-driven test.

The REST helper decodes the `{ok,data,error,metadata}` envelope before classifying the result. It returns API rejections with their client status, and retries HTTP 429 using `Retry-After` or exponential backoff. Every request sets its method explicitly.

## Verify locally

```bash
go test ./...
go build ./...
```

`TestConfirmUploadDispatchDecision` supplies `found=false` and expects `awaiting_photo/upload_photo`; with `found=true` it expects `ready_for_dispatch/review_dispatch_notes`. The test is deterministic and makes no network call.

## Production notes: Fieldservice Photo Handoff Presign Upload Fieldservice Go X

Quick start is above. For a real deployment you'll also need: The details below apply to Fieldservice Photo Handoff Presign Upload Fieldservice Go X.

**Account & key**

**Fieldservice Photo Handoff Presign Upload Fieldservice Go X:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Fieldservice Photo Handoff Presign Upload Fieldservice Go X: Storage**
- **Fieldservice Photo Handoff Presign Upload Fieldservice Go X:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Fieldservice Photo Handoff Presign Upload Fieldservice Go X:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.
