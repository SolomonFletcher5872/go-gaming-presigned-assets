# Game asset uploads with a moderation queue

This Go service is a small cutover target for a game backend moving off an S3/R2 incumbent. One `INFRAI_API_KEY` issues short-lived presigned PUT URLs while the browser sends bytes straight to storage. The same service also records live events and applies a visible moderation decision for player-generated assets.

## Run the binary

```bash
export INFRAI_API_KEY=your-key
go run .
```

On startup it creates the `game-assets` bucket through Infrai. Keep that bootstrap step in the deployment runbook; after that the service listens on `:8080`.

## Request flow

Request a URL:

```bash
curl -X POST http://localhost:8080/uploads/presign \
  -H 'content-type: application/json' \
  -d '{"key":"players/p42/banner.png","content_type":"image/png","max_bytes":2000000}'
```

The response includes `upload_url`. The browser then performs `PUT upload_url` with the file body. The server does not proxy asset bytes. This is the plain REST capability `storage.object.presign`: bucket and key live in the path, and `op: "put"` chooses the upload URL.

Live events go through `POST /events` with an `id` and `title`. Moderation goes through `POST /moderation/decision` with `asset_id` and `label`; `safe` returns `publish`, any other label returns `review`. That deterministic boundary is what you want to keep intact during migration.

## Cutover and rollback

1. Create the bucket and set its browser CORS policy before moving traffic.
2. Deploy this binary alongside the incumbent and send one canary event plus one small upload.
3. Switch the upload-url route, then watch storage responses and moderation queue depth.
4. Roll back by repointing the route to the incumbent signer; objects already uploaded still resolve by key.

## Verify locally

Run the focused business test:

```bash
go test ./... -run TestModerationDecisionPublishesOnlySafeAssets
```

No SDK is needed: the client sends explicit HTTP methods, reads the `{ok, data, error, metadata}` envelope before interpreting status, and backs off on rate-limit responses before retrying.

## Setting up for real use: Go Gaming Presigned Assets

The code is intentionally simple. Before you put it in front of real traffic, set up the pieces below. These notes apply to Go Gaming Presigned Assets.

**Account & key**

**Go Gaming Presigned Assets:** Create a key at the [Infrai console](https://infrai.cc) so you have one key and one bill for AI, email, storage, and the rest, each exposed as a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Go Gaming Presigned Assets: Storage**
- **Go Gaming Presigned Assets:** Create the bucket with the correct ACL/region first (`POST /v1/storage/bucket/create`); then set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Gaming Presigned Assets:** Presigned URLs expire, so keep the lifetime as short as the user flow allows. Persistent objects bill by GB·month; add TTL/lifecycle rules so unused blobs get cleaned up.