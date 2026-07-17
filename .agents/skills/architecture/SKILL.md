---
name: architecture
description: "Project architecture document. Use when you need to understand the system architecture, components, or data flow when implementing or updating features."
---

# Blog Comments API - Architecture Document

This document outlines the architecture for the Blog Comments API service, which provides REST endpoints for managing blog comments with anti-spam protection.

We split this document into multiple files to make it easier to navigate and maintain. Ready **only** what's needed for the current task.

---

## Table of Contents

1. [System Overview](#system-overview)
2. [API Specification](#api-specification)
3. [Data Architecture](#data-architecture)
4. [Security Architecture](#security-architecture)
5. [Component Architecture](#component-architecture)
6. [Error Handling](#error-handling)
7. [Observability](#observability)
8. [Testing Strategy](#testing-strategy)
9. [Deployment Architecture](#deployment-architecture)
10. [Implementation Roadmap](#implementation-roadmap)

---

## System Overview

Read `overview.md` when your task involves any of the following:

- **System purpose** — understanding what the service does and its high-level data flow
- **Core constraints** — SQLite, Cloudflare Turnstile, visibility rules, concurrency model, CORS configuration
- **Database Identifier strategy** — `id` (integer PK) vs `display_id` (hashid-derived), and which to use in what context

---

## API Specification

Read `api-specification.md` when your task involves any of the following:

- **Adding or modifying endpoints** — creating, reading, listing, or deleting comments
- **Request/response formats** — JSON structures, field names, data types, and pagination shape
- **Validation rules** — required fields, max lengths, format constraints per endpoint
- **Query parameters** — pagination, filtering, sorting parameters accepted by list endpoints
- **Error responses** — status codes and error scenarios specific to each endpoint
- **CORS configuration** — allowed origins, methods, headers, and preflight behavior

## Data Architecture

Read `database.md` when your task involves any of the following:

- **Database schema** — table structure, column semantics, indexes, and schema evolution
- **Display ID generation** — HashID strategy, salt configuration, and deterministic generation from the integer PK
- **Data flow** — comment creation flow (validation → rate limit → Turnstile → insert → display_id update → response) and retrieval flow
- **Concurrency strategy** — mutex-based write serialization, SQLite WAL mode for concurrent reads, and the rationale compared to connection pooling
- **SQLite configuration** — WAL mode, busy timeout, foreign keys, and synchronous mode settings

## Security Architecture

Read `security.md` when your task involves any of the following:

- **Input sanitization** — HTML stripping, Unicode normalization (NFC), null byte removal, anti-XSS measures
- **Rate limiting** — per-IP rate limiting configuration, burst settings, visitor cleanup strategy
- **Turnstile verification** — Cloudflare Turnstile integration, token verification flow, caching of verification results
- **CORS configuration** — allowed origins, methods, headers, preflight handling, environment-based configuration
- **Data protection** — what data is stored, logged, and returned in API responses; PII handling and logging restrictions

## Component Architecture

Read `components.md` when your task involves any of the following:

- **Project structure** — understanding the directory layout, where to place new files, and how the codebase is organized
- **Component interfaces and responsibilities** — handler, service, repository, and domain model contracts and how they interact
- **Middleware chain** — the order of middleware (recovery, logging, CORS, rate limiting) and request flow through the stack
- **Dependency injection** — how components are wired together in `main()`, dependency graph, and initialization order
- **Adding or modifying components** — creating new handlers, services, repositories, middleware, or domain types; understanding which layer to extend

## Error Handling

Read `error-handling.md` when your task involves any of the following:

- **Error taxonomy** — understanding error codes, HTTP status mappings, and when each error type is used
- **Error response format** — JSON structure for error responses, field error details, and consistency across endpoints
- **Error handling implementation** — the `writeError` helper function, usage patterns in handlers, and error propagation
- **Logging errors** — structured error logging conventions, what to include in log context, and data protection when logging

## Observability

Read `observability.md` when your task involves any of the following:

- **Health check endpoint** — implementing or modifying the `/health` endpoint, response shape, and dependency checks
- **Metrics** — adding or updating metrics counters/histograms, metric naming conventions, and exposure endpoints
- **Structured logging** — JSON logging configuration, context fields to include in log entries, and consistent log key naming
- **Monitoring alerts** — alert thresholds for error rates, Turnstile failures, query latency, and abuse detection

## Testing Strategy

Read `testing.md` when your task involves any of the following:

- **Test pyramid** — understanding the distribution of unit, integration, and E2E tests
- **Unit tests** — table-driven test patterns, model validation tests, sanitizer tests, display ID tests
- **Integration tests** — in-memory SQLite setup, repository tests, handler tests with real database
- **Test infrastructure** — test helpers, database setup/teardown, migration execution in tests
