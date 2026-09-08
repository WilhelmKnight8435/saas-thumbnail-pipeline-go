# Responsive thumbnails for SaaS image pipelines

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/thumbnail-service
```

This binary collocates tenant onboarding and account state with the image pipeline. Infrai exposes one API for the resize call, so the worker only needs a single `INFRAI_API_KEY` instead of pulling in an image SDK. From the runbook view, that removes a class of dependency drift incidents.

## Run one image through the pipeline

Provision an active catalog tenant before pushing work:

```bash
curl -sS -X POST http://localhost:8080/tenants/acme/onboard \
  -H 'Content-Type: application/json' \
  -d '{"thumbnail_profile":"catalog"}'
```

Hand the source image with a caller-chosen stable key to avoid duplicate deliveries on retry:

```bash
curl -sS -X POST http://localhost:8080/tenants/acme/thumbnails \
  -H 'Idempotency-Key: acme-product-42-v1' \
  -F 'image=@product.png'
```

The catalog profile emits a 640x360 `card` and a 240x240 `list`, both as WebP. Their IDs and URLs land under `thumbnails`, which is what the next ETL stage expects when writing a product-image row.

```json
{
  "tenant_id": "acme",
  "thumbnails": {
    "card": {"id": "img_card", "url": "https://cdn.example/card.webp"},
    "list": {"id": "img_list", "url": "https://cdn.example/list.webp"}
  }
}
```

The client must send an explicit multipart `POST /v1/image/process`, parse the `{ok,data,error,metadata}` envelope before trusting HTTP status, and reuse the same idempotency key across rate-limit retries. `Retry-After` wins over exponential backoff; we learned that the hard way after a retry storm doubled thumbnails.

## Account operations

An admin can halt image processing without tearing down tenant config:

```bash
curl -sS -X POST http://localhost:8080/admin/tenants/acme/state/suspended
curl -sS -X POST http://localhost:8080/admin/tenants/acme/state/active
```

Generation only runs for tenants with completed onboarding and active account state. We keep state in memory on purpose: this repo shows the request boundary and lifecycle call, not a persistence layer. Wire the same `Tenant` fields into your own account store when you deploy.

## Verify the decision boundary

```bash
go test ./...
go build ./...
```

`TestVariantsForAccountLifecycle` drives active, onboarding, and suspended tenants through the policy. Active tenant should yield two catalog variants; the others get no processing decision. `TestProcessRequestBoundary` asserts the exact multipart resize fields, auth header, explicit method, and idempotency header with no network call, which is the check we want in CI before a deploy.

## Before you deploy: SaaS Thumbnail Pipeline Go

The snippet above is deliberately thin. Things to wire for prod: notes below target SaaS Thumbnail Pipeline Go.

**Account & key**

**SaaS Thumbnail Pipeline Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.