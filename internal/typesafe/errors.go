package typesafe

import (
	"errors"
	"fmt"
)

const (
	ReasonInvalidKey         = "invalid_key"
	ReasonRequestRejected    = "request_rejected"
	ReasonRateLimited        = "rate_limited"
	ReasonProviderOverloaded = "provider_overloaded"
	ReasonProviderError      = "provider_error"
	ReasonUnexpectedRedirect = "unexpected_redirect"
	ReasonTimeout            = "timeout"
	ReasonCancelled          = "cancelled"
	ReasonNetworkError       = "network_error"
	ReasonMalformedResponse  = "malformed_response"
	ReasonRequestTooLarge    = "request_too_large"
)

// Error is a path-free, secret-free provider failure.
type Error struct {
	Reason string
	Status int
}

func (e *Error) Error() string {
	if e == nil {
		return "typesafe: unknown error"
	}
	if e.Status > 0 {
		return fmt.Sprintf("typesafe: %s (HTTP %d)", e.Reason, e.Status)
	}
	return "typesafe: " + e.Reason
}

// ReasonOf returns a stable reason code for err, or "" when err is not a typesafe.Error.
func ReasonOf(err error) string {
	var typed *Error
	if errors.As(err, &typed) && typed != nil {
		return typed.Reason
	}
	return ""
}

func typedError(reason string, status int) *Error {
	return &Error{Reason: reason, Status: status}
}
