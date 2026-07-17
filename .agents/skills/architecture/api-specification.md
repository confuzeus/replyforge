# API Specification

## Base URL

```text
https://api.example.com/api/v1
```

### Common Headers

```text
Content-Type: application/json
Accept: application/json
```

### Endpoints

#### 1. Create Comment

```text
POST /api/v1/comments
```

**Request Body:**

```json
{
  "post_id": "my-blog-post",
  "author_name": "Jane Doe",
  "body": "Great article! This helped me understand the topic.",
  "turnstile_token": "XXXX.DUMMY.TOKEN.XXXX"
}
```

**Field Validation:**

| Field             | Type   | Required | Constraints               |
| ----------------- | ------ | -------- | ------------------------- |
| `post_id`         | string | Yes      | Non-empty, max 100 chars  |
| `author_name`     | string | Yes      | Non-empty, max 100 chars  |
| `body`            | string | Yes      | Non-empty, max 5000 chars |
| `turnstile_token` | string | Yes      | Non-empty                 |

**Response (201 Created):**

```json
{
  "data": {
    "id": 42,
    "display_id": "xj3k9p",
    "post_id": "my-blog-post",
    "author_name": "Jane Doe",
    "body": "Great article! This helped me understand the topic.",
    "approved": false,
    "created_at": "2026-07-16T10:30:00Z"
  }
}
```

**Error Responses:**

- `400 Bad Request` - Validation errors
- `403 Forbidden` - Turnstile verification failed
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Unexpected server error

#### 2. List Comments

```text
GET /api/v1/comments?post_id={post_id}&page={page}&per_page={per_page}&sort={sort}
```

**Query Parameters:**

| Parameter  | Type    | Required | Default       | Description                               |
| ---------- | ------- | -------- | ------------- | ----------------------------------------- |
| `post_id`  | string  | No       | None          | Filter by blog post                       |
| `page`     | integer | No       | 1             | Page number (min 1)                       |
| `per_page` | integer | No       | 20            | Items per page (min 1, max 100)           |
| `sort`     | string  | No       | `-created_at` | Sort field: `created_at` or `-created_at` |

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": 42,
      "display_id": "xj3k9p",
      "post_id": "my-blog-post",
      "author_name": "Jane Doe",
      "body": "Great article! This helped me understand the topic.",
      "approved": true,
      "created_at": "2026-07-16T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

**Business Rules:**

- Only returns comments where `approved = true`
- Default sort is newest first (`-created_at`)
- Returns empty array with pagination metadata when no comments exist
- Both `id` and `display_id` are included in each comment object

**Example Requests:**

```
GET /api/v1/comments                                          # All approved comments
GET /api/v1/comments?post_id=my-blog-post                    # Comments for specific post
GET /api/v1/comments?post_id=my-blog-post&page=2&per_page=10 # Paginated
GET /api/v1/comments?post_id=my-blog-post&sort=created_at    # Oldest first
```

#### 3. Get Single Comment

```text
GET /api/v1/comments/{id}
```

**Path Parameters:**

| Parameter | Type    | Description                          |
| --------- | ------- | ------------------------------------ |
| `id`      | integer | The real (auto-increment) comment ID |

**Response (200 OK):**

```json
{
  "data": {
    "id": 42,
    "display_id": "xj3k9p",
    "post_id": "my-blog-post",
    "author_name": "Jane Doe",
    "body": "Great article! This helped me understand the topic.",
    "approved": true,
    "created_at": "2026-07-16T10:30:00Z"
  }
}
```

**Error Responses:**

- `404 Not Found` - Comment not found or not approved
- `400 Bad Request` - Invalid ID format (non-integer)

#### 4. CORS Preflight

```text
OPTIONS /api/v1/comments
OPTIONS /api/v1/comments/{id}
```

**Response Headers:**

```text
Access-Control-Allow-Origin: https://exampleblog.com
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type
Access-Control-Max-Age: 86400
```
