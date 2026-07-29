package health

import "time"

// response is the JSON payload returned by the health endpoint.
type response struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}

// healthOutput is the Huma operation output. Huma marshals the Body field as the
// JSON response body, so the wire format stays {"status":"ok","time":"..."}.
type healthOutput struct {
	Body response
}
