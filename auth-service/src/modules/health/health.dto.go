package health

import "time"

// response is the JSON payload returned by the health endpoint.
type response struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}
