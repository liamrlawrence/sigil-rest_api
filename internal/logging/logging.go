package logging

import (
	"log"
	"net/http"
)

func APIEndpoint(r *http.Request, verb string, endpoint string) {
	sessionID := r.Header.Get("X-Grimoire-Token")
	if sessionID == "" {
		sessionID = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	}
	log.Printf("%s %4s %s\n", sessionID, verb, endpoint)
}
