package kite

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// Kind is the error taxonomy of ARCHITECTURE §5.3.
type Kind uint8

const (
	KindNone        Kind = iota
	KindSession          // TokenException: session expired, all queues pause, no retry
	KindPermission       // PermissionError: degraded mode, no retry
	KindRateLimited      // HTTP 429: back-off on that class only
	KindNetwork          // NetworkException / transport failure: idempotent GETs retry ≤ 3×
	KindInput            // InputException / DataException (e.g. historical window too large)
	KindOrder            // OrderException
	KindServer           // GeneralException / 5xx: GETs retry once after 2 s
	KindCanceled         // context cancelled or deadline exceeded
	KindUnknown
)

var kindNames = map[Kind]string{
	KindNone: "none", KindSession: "session", KindPermission: "permission", KindRateLimited: "rate-limited",
	KindNetwork: "network", KindInput: "input", KindOrder: "order", KindServer: "server",
	KindCanceled: "canceled", KindUnknown: "unknown",
}

func (k Kind) String() string { return kindNames[k] }

// Err is the error every Client method returns. Message is Kite's text (never a header).
type Err struct {
	Kind    Kind
	Op      string // ReadClient method name, e.g. "Historical"
	Code    int    // HTTP status (0 when no response)
	Type    string // Kite error_type
	Message string
	cause   error
}

func (e *Err) Error() string {
	return fmt.Sprintf("kite %s: %s (%s, %d %s)", e.Op, e.Message, e.Kind, e.Code, e.Type)
}

func (e *Err) Unwrap() error { return e.cause }

// IsOffline reports whether err means Kite could not be reached (NFR-14 offline badge).
func IsOffline(err error) bool { return Classify(err) == KindNetwork }

// Classify maps any error from gokiteconnect, the transport or this package to a Kind.
func Classify(err error) Kind {
	if err == nil {
		return KindNone
	}
	var ke *Err
	if errors.As(err, &ke) {
		return ke.Kind
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return KindCanceled
	}
	var e kiteconnect.Error
	if !errors.As(err, &e) {
		return KindUnknown
	}
	if e.Code == http.StatusTooManyRequests {
		return KindRateLimited
	}
	switch e.ErrorType {
	case kiteconnect.TokenError:
		return KindSession
	case kiteconnect.PermissionError:
		return KindPermission
	case kiteconnect.NetworkError:
		return KindNetwork
	case kiteconnect.OrderError:
		return KindOrder
	case kiteconnect.InputError:
		return KindInput
	case kiteconnect.DataError:
		// gokiteconnect also uses DataException (code 504) for bodies it cannot read or parse
		// (http.go readEnvelope); those are server faults, not window errors.
		if e.Code >= 500 {
			return KindServer
		}
		return KindInput
	}
	if e.Code >= 500 || e.ErrorType == kiteconnect.GeneralError {
		return KindServer
	}
	return KindUnknown
}

func wrap(op string, err error) *Err {
	if err == nil {
		return nil
	}
	var ke *Err
	if errors.As(err, &ke) {
		return ke
	}
	out := &Err{Kind: Classify(err), Op: op, Message: err.Error(), cause: err}
	var e kiteconnect.Error
	if errors.As(err, &e) {
		out.Code, out.Type, out.Message = e.Code, e.ErrorType, e.Message
	}
	return out
}
