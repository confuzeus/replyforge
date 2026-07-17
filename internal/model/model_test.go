package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentResponse_JSONRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 7, 16, 10, 30, 0, 0, time.UTC)
	cr := CommentResponse{
		ID:         42,
		DisplayID:  "xj3k9p",
		PostID:     "my-blog-post",
		AuthorName: "Jane Doe",
		Body:       "Great article! This helped me understand the topic.",
		Approved:   false,
		CreatedAt:  createdAt,
	}

	jsonBytes, err := json.Marshal(cr)
	require.NoError(t, err)

	var decoded CommentResponse
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, int64(42), decoded.ID)
	assert.Equal(t, "xj3k9p", decoded.DisplayID)
	assert.Equal(t, "my-blog-post", decoded.PostID)
	assert.Equal(t, "Jane Doe", decoded.AuthorName)
	assert.Equal(t, "Great article! This helped me understand the topic.", decoded.Body)
	assert.False(t, decoded.Approved)
}

func TestCreateCommentRequest_JSONRoundTrip(t *testing.T) {
	req := CreateCommentRequest{
		PostID:         "my-blog-post",
		AuthorName:     "Jane Doe",
		Body:           "Great article!",
		TurnstileToken: "XXXX.DUMMY.TOKEN.XXXX",
	}

	jsonBytes, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded CreateCommentRequest
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "my-blog-post", decoded.PostID)
	assert.Equal(t, "Jane Doe", decoded.AuthorName)
	assert.Equal(t, "Great article!", decoded.Body)
	assert.Equal(t, "XXXX.DUMMY.TOKEN.XXXX", decoded.TurnstileToken)
}

func TestListCommentsResponse_JSONRoundTrip(t *testing.T) {
	resp := ListCommentsResponse{
		Data: []*CommentResponse{
			{
				ID:         42,
				DisplayID:  "xj3k9p",
				PostID:     "my-blog-post",
				AuthorName: "Jane Doe",
				Body:       "Great article!",
				Approved:   true,
				CreatedAt:  time.Date(2026, 7, 16, 10, 30, 0, 0, time.UTC),
			},
		},
		Pagination: PaginationMeta{
			Page:       1,
			PerPage:    20,
			Total:      1,
			TotalPages: 1,
		},
	}

	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded ListCommentsResponse
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Data, 1)
	assert.Equal(t, int64(42), decoded.Data[0].ID)
	assert.Equal(t, 1, decoded.Pagination.Page)
	assert.Equal(t, 20, decoded.Pagination.PerPage)
	assert.Equal(t, 1, decoded.Pagination.Total)
	assert.Equal(t, 1, decoded.Pagination.TotalPages)
}

func TestCreateCommentRequest_Validate(t *testing.T) {
	tests := []struct {
		name      string
		request   CreateCommentRequest
		wantErr   bool
		errFields []string
	}{
		{
			name: "valid request",
			request: CreateCommentRequest{
				PostID:         "my-post",
				AuthorName:     "Jane Doe",
				Body:           "Great article!",
				TurnstileToken: "valid-token",
			},
			wantErr: false,
		},
		{
			name: "missing post_id",
			request: CreateCommentRequest{
				PostID:         "",
				AuthorName:     "Jane Doe",
				Body:           "Great article!",
				TurnstileToken: "valid-token",
			},
			wantErr:   true,
			errFields: []string{"post_id"},
		},
		{
			name: "missing author_name",
			request: CreateCommentRequest{
				PostID:         "my-post",
				AuthorName:     "",
				Body:           "Great article!",
				TurnstileToken: "valid-token",
			},
			wantErr:   true,
			errFields: []string{"author_name"},
		},
		{
			name: "missing body",
			request: CreateCommentRequest{
				PostID:         "my-post",
				AuthorName:     "Jane Doe",
				Body:           "",
				TurnstileToken: "valid-token",
			},
			wantErr:   true,
			errFields: []string{"body"},
		},
		{
			name: "missing turnstile_token",
			request: CreateCommentRequest{
				PostID:         "my-post",
				AuthorName:     "Jane Doe",
				Body:           "Great article!",
				TurnstileToken: "",
			},
			wantErr:   true,
			errFields: []string{"turnstile_token"},
		},
		{
			name: "post_id exceeds max length",
			request: CreateCommentRequest{
				PostID:         strings.Repeat("a", 101),
				AuthorName:     "Jane Doe",
				Body:           "Great article!",
				TurnstileToken: "valid-token",
			},
			wantErr:   true,
			errFields: []string{"post_id"},
		},
		{
			name: "post_id at max length",
			request: CreateCommentRequest{
				PostID:         strings.Repeat("a", 100),
				AuthorName:     "Jane Doe",
				Body:           "Great article!",
				TurnstileToken: "valid-token",
			},
			wantErr: false,
		},
		{
			name: "author_name exceeds max length",
			request: CreateCommentRequest{
				PostID:         "my-post",
				AuthorName:     strings.Repeat("a", 101),
				Body:           "Great article!",
				TurnstileToken: "valid-token",
			},
			wantErr:   true,
			errFields: []string{"author_name"},
		},
		{
			name: "author_name at max length",
			request: CreateCommentRequest{
				PostID:         "my-post",
				AuthorName:     strings.Repeat("a", 100),
				Body:           "Great article!",
				TurnstileToken: "valid-token",
			},
			wantErr: false,
		},
		{
			name: "body exceeds max length",
			request: CreateCommentRequest{
				PostID:         "my-post",
				AuthorName:     "Jane Doe",
				Body:           strings.Repeat("a", 5001),
				TurnstileToken: "valid-token",
			},
			wantErr:   true,
			errFields: []string{"body"},
		},
		{
			name: "body at max length",
			request: CreateCommentRequest{
				PostID:         "my-post",
				AuthorName:     "Jane Doe",
				Body:           strings.Repeat("a", 5000),
				TurnstileToken: "valid-token",
			},
			wantErr: false,
		},
		{
			name: "all fields empty",
			request: CreateCommentRequest{
				PostID:         "",
				AuthorName:     "",
				Body:           "",
				TurnstileToken: "",
			},
			wantErr:   true,
			errFields: []string{"post_id", "author_name", "body", "turnstile_token"},
		},
		{
			name: "multiple fields over limit",
			request: CreateCommentRequest{
				PostID:         strings.Repeat("a", 101),
				AuthorName:     strings.Repeat("b", 101),
				Body:           strings.Repeat("c", 5001),
				TurnstileToken: "valid-token",
			},
			wantErr:   true,
			errFields: []string{"post_id", "author_name", "body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if !tt.wantErr {
				assert.Nil(t, err)
				return
			}

			require.NotNil(t, err)
			var fieldNames []string
			for _, f := range err.Fields {
				fieldNames = append(fieldNames, f.Field)
			}
			assert.ElementsMatch(t, tt.errFields, fieldNames)
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{
		Fields: []FieldError{
			{Field: "post_id", Message: "is required"},
			{Field: "body", Message: "must be at most 5000 characters"},
		},
	}

	errMsg := ve.Error()
	assert.Contains(t, errMsg, "post_id: is required")
	assert.Contains(t, errMsg, "body: must be at most 5000 characters")
	assert.Contains(t, errMsg, "; ")
}

func TestErrorResponse_JSONRoundTrip(t *testing.T) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    "validation_error",
			Message: "Invalid request parameters",
			Details: []FieldError{
				{Field: "post_id", Message: "is required"},
				{Field: "author_name", Message: "is required"},
			},
		},
	}

	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	assert.Contains(t, string(jsonBytes), `"code":"validation_error"`)
	assert.Contains(t, string(jsonBytes), `"field":"post_id"`)
	assert.Contains(t, string(jsonBytes), `"field":"author_name"`)
}

func TestErrorResponse_JSONWithoutDetails(t *testing.T) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    "internal_error",
			Message: "Something went wrong",
		},
	}

	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	assert.NotContains(t, string(jsonBytes), "details")
}

func TestPaginationMeta_JSONRoundTrip(t *testing.T) {
	meta := PaginationMeta{
		Page:       2,
		PerPage:    10,
		Total:      25,
		TotalPages: 3,
	}

	jsonBytes, err := json.Marshal(meta)
	require.NoError(t, err)

	assert.Contains(t, string(jsonBytes), `"page":2`)
	assert.Contains(t, string(jsonBytes), `"per_page":10`)
	assert.Contains(t, string(jsonBytes), `"total":25`)
	assert.Contains(t, string(jsonBytes), `"total_pages":3`)
}
