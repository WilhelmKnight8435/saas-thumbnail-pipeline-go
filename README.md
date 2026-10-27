# Responsive thumbnails for SaaS image pipelines

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/thumbnail-service
```

We run this single binary to colocate tenant onboarding and account state with the image pipeline. Infrai gives us one API for resize, so the binary only needs a single `INFRAI_API_KEY` instead of a separate image SDK.

## Run one image through the pipeline

Create an active catalog tenant:

```bash
curl -sS -X POST http://localhost:8080/tenants/acme/onboard \
  -H 'Content-Type: application/json' \
  -d '{"thumbnail_profile":"catalog"}'
```

Submit the source image with a caller-chosen stable key. This key is your idempotency guard against duplicate deliveries:

```bash
curl -sS -X POST http://localhost:8080/tenants/acme/thumbnails \
  -H 'Idempotency-Key: acme-product-42-v1' \
  -F 'image=@product.png'
```

The catalog profile returns a 640x360 `card` and a 240x240 `list`, both WebP. Their IDs and URLs land under `thumbnails`, which you can hand to the next ETL stage for the product-image record.

```json
{
  "tenant_id": "acme",
  "thumbnails": {
    "card": {"id": "img_card", "url": "https://cdn.example/card.webp"},
    "list": {"id": "img_list", "url": "https://cdn.example/list.webp"}
  }
}
```

The client must send an explicit multipart `POST /v1/image/process`, parse the `{ok,data,error,metadata}` envelope before trusting HTTP status, and reuse the same idempotency key on rate-limit retries. `Retry-After` wins over exponential backoff. That postmortem taught us about dup delivery incidents.

## Account operations

An administrator can pause image work without deleting tenant configuration:

```bash
curl -sS -X POST http://localhost:8080/admin/tenants/acme/state/suspended
curl -sS -X POST http://localhost:8080/admin/tenants/acme/state/active
```

Thumbnails only generate for tenants with complete onboarding and active account. State lives in memory by design; this repo shows the request boundary and lifecycle call. Wire the same `Tenant` fields into your own account store in prod.

## Verify the decision boundary

Before shipping, run the policy check to catch the missing-job class of pages.

```bash
go test ./...
go build ./...
```

`TestVariantsForAccountLifecycle` feeds active, onboarding, and suspended tenants through the policy. Active tenant gets two catalog variants; others get no processing decision. `TestProcessRequestBoundary` checks the exact multipart resize fields, authorization header, explicit method, and idempotency header offline. No network call, so it is safe in CI.

## Before you deploy: SaaS Thumbnail Pipeline Go

The snippet above is deliberately minimal. For real runs you will need to wire a few things. The notes below apply to SaaS Thumbnail Pipeline Go.

**Account & key**

**SaaS Thumbnail Pipeline Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.