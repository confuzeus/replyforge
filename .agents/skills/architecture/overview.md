# System Overview

## Purpose

A REST API service for managing blog comments with anti-spam protection via Cloudflare Turnstile. The system stores comments in SQLite and serves approved comments to readers while protecting against automated spam.

### Core Constraints

- **Database:** SQLite (single-file, no external database server)
- **Anti-spam:** Cloudflare Turnstile verification (server-side)
- **Visibility:** Only approved comments are publicly accessible
- **Concurrency:** Multiple readers, serialized writers (SQLite constraint)
- **Cross-Origin:** Explicit CORS configuration for blog domains

### Architecture Diagram

```text
┌─────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Browser   │────▶│   Go HTTP Server  │────▶│    SQLite DB    │
│  (Blog)     │     │   (net/http)      │     │   (comments.db) │
└─────────────┘     └──────────────────┘     └─────────────────┘
                           │
                           ▼
                    ┌──────────────────┐
                    │    Cloudflare    │
                    │    Turnstile     │
                    │   Verification   │
                    └──────────────────┘
```

### Identifier Strategy

The system uses two identifiers for each comment, serving different purposes:

| Identifier   | Type                    | Format   | Purpose                                     | Exposed                |
| ------------ | ----------------------- | -------- | ------------------------------------------- | ---------------------- |
| `id`         | Integer, auto-increment | `42`     | Primary key, database performance, API URLs | Yes (responses + URLs) |
| `display_id` | String, HashID-derived  | `xj3k9p` | User-facing reference, frontend display     | Yes (responses only)   |

**Design Rationale:**

- Integer `id` provides optimal SQLite performance for primary key lookups and index operations
- `display_id` gives users a short, non-sequential identifier for referencing comments
- Display IDs are not guaranteed globally unique but are unique enough within the context of a blog post
- Real IDs never leak sequential information through user-facing displays

**Frontend Display Example:**

```text
Jane Doe · 2 hours ago · #xj3k9p
Great article! This helped me understand the topic.
```
