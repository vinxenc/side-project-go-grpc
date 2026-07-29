package health

import (
	"net/http"
	"time"

	"auth-service/core"
)

// controller holds the health endpoint handlers.
// It embeds core.BaseController to reuse the shared response helpers.
type controller struct {
	core.BaseController
}

// health reports whether the service is up.
func (c *controller) health(w http.ResponseWriter, r *http.Request) {
	c.JSON(w, http.StatusOK, response{
		Status: "ok",
		Time:   time.Now().UTC(),
	})
}
