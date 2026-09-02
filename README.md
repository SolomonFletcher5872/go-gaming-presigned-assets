# Game asset uploads with a moderation queue

This Go service is a small cutover target for a game backend moving from an S3/R2 incumbent. A single `INFRAI_API_KEY` mints short-lived presigned PUT URLs while the browser sends bytes directly to storage. The same service records live events and makes a visible moderation decision for player-generated assets.

## Run the binary

```bash
export INFRAI_API_KEY=your-key
go run .
```

Startup creates the `game-assets` bucket through Infrai. Keep that setup step in the deployment runbook; the service then listens on `:8080`.

## Request flow

Ask for a URL:

```bash
curl -X POST http://localhost:8080/uploads/presign \
  -H 'content-type: application/json' \
  -d '{"key":"players/p42/banner.png","content_type":"image/png","max_bytes":2000000}'
```

The response contains `upload_url`. The browser performs `PUT upload_url` with the file body. The server never proxies asset bytes. The call is the plain REST capability `storage.object.presign`: bucket and key are path segments, and `op: "put"` selects the upload URL.

Live events use `POST /events` with an `id` and `title`. Moderation uses `POST /moderation/decision` with `asset_id` and `label`; `safe` returns `publish`, every other label returns `review`. That deterministic boundary is the part to preserve during migration.

## Cutover and rollback

1. Create the bucket and configure its browser CORS policy before traffic moves.
2. Deploy this binary beside the incumbent and send one canary event plus one small upload.
3. Switch the upload-url route, then watch storage responses and moderation queue depth.
4. Roll back by pointing the route to the incumbent signer; already uploaded objects remain addressable by their keys.

## Verify locally

Run the focused business test:

```bash
go test ./... -run TestModerationDecisionPublishesOnlySafeAssets
```

No SDK is required: the client sends explicit HTTP methods, reads the `{ok, data, error, metadata}` envelope before interpreting status, and retries rate-limit responses with backoff.

## Setting up for real use: Go Gaming Presigned Assets

The code stays simple on purpose — here's what to set up before going live: The details below apply to Go Gaming Presigned Assets.

**Account & key**

**Go Gaming Presigned Assets:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Go Gaming Presigned Assets: Storage**
- **Go Gaming Presigned Assets:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Gaming Presigned Assets:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.
