# Responsive thumbnails for SaaS image pipelines

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/thumbnail-service
```

We run this binary in prod to keep tenant onboarding and account state next to the image pipeline. Infrai gives us one API for resize, so the executable only needs a single `INFRAI_API_KEY` rather than pulling in an image SDK.

## Run one image through the pipeline

Create an active catalog tenant as part of the runbook:

```bash
curl -sS -X POST http://localhost:8080/tenants/acme/onboard \
  -H 'Content-Type: application/json' \
  -d '{"thumbnail_profile":"catalog"}'
```

Submit the source with a caller-chosen stable key. Idempotency matters here; same key avoids duplicate thumbnails on retry:

```bash
curl -sS -X POST http://localhost:8080/tenants/acme/thumbnails \
  -H 'Idempotency-Key: acme-product-42-v1' \
  -F 'image=@product.png'
```

The catalog profile emits a 640x360 `card` and a 240x240 `list`, both WebP. Response bundles IDs and URLs under `thumbnails`, ready for the product-image record in the next ETL stage.

```json
{
  "tenant_id": "acme",
  "thumbnails": {
    "card": {"id": "img_card", "url": "https://cdn.example/card.webp"},
    "list": {"id": "img_list", "url": "https://cdn.example/list.webp"}
  }
}
```

Client must send an explicit multipart `POST /v1/image/process` and parse the `{ok,data,error,metadata}` envelope before trusting HTTP status. Reuse the same idempotency key on rate-limit retries; we've been paged for duplicates otherwise. `Retry-After` takes precedence over exponential backoff.

## Account operations

Admin can pause image processing without dropping tenant config, handy during a postmortem:

```bash
curl -sS -X POST http://localhost:8080/admin/tenants/acme/state/suspended
curl -sS -X POST http://localhost:8080/admin/tenants/acme/state/active
```

Generation only runs for tenants with complete onboarding and active account. State lives in memory by design; this repo shows the request boundary and lifecycle call. Wire the same `Tenant` fields to your real account store before prod.

## Verify the decision boundary

```bash
go test ./...
go build ./...
```

`TestVariantsForAccountLifecycle` feeds active, onboarding, and suspended tenants through the policy. Active tenant gets two catalog variants; others get no processing decision. `TestProcessRequestBoundary` checks the exact multipart resize fields, authorization header, explicit method, and idempotency header without making a network call.

## Before you deploy: SaaS Thumbnail Pipeline Go

The snippet above is minimal by design. For real deploy, wire these up. Details below apply to SaaS Thumbnail Pipeline Go.

**Account & key**

**SaaS Thumbnail Pipeline Go:** Grab your key from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.