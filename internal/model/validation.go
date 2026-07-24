package model

const (
	maxPostIDLen     = 100
	maxAuthorNameLen = 100
	maxBodyLen       = 5000
)

func (r *CreateCommentRequest) Validate() *ValidationError {
	var fields []FieldError

	if r.PostID == "" {
		fields = append(fields, FieldError{
			Field:   "post_id",
			Message: "is required",
		})
	} else if len(r.PostID) > maxPostIDLen {
		fields = append(fields, FieldError{
			Field:   "post_id",
			Message: "must be at most 100 characters",
		})
	}

	if r.AuthorName == "" {
		fields = append(fields, FieldError{
			Field:   "author_name",
			Message: "is required",
		})
	} else if len(r.AuthorName) > maxAuthorNameLen {
		fields = append(fields, FieldError{
			Field:   "author_name",
			Message: "must be at most 100 characters",
		})
	}

	if r.Body == "" {
		fields = append(fields, FieldError{
			Field:   "body",
			Message: "is required",
		})
	} else if len(r.Body) > maxBodyLen {
		fields = append(fields, FieldError{
			Field:   "body",
			Message: "must be at most 5000 characters",
		})
	}

	if len(fields) == 0 {
		return nil
	}

	return &ValidationError{Fields: fields}
}
