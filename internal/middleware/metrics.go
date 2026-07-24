package middleware

import "expvar"

var (
	CommentsCreatedTotal      = expvar.NewInt("comments_created_total")
	CaptchaVerificationsTotal = expvar.NewInt("captcha_verifications_total")
	CaptchaFailedTotal        = expvar.NewInt("captcha_verifications_failed_total")
	RateLimitHitsTotal        = expvar.NewInt("rate_limit_hits_total")
	ValidationErrorsTotal     = expvar.NewInt("validation_errors_total")
	PanicsTotal               = expvar.NewInt("panics_total")
)
