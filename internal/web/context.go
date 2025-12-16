package web

import "net/http"

type RequestContext struct {
	IsHTMX      bool   // HX-Request header present
	CurrentURL  string // HX-Current-URL - where the user is
	TriggerID   string // HX-Trigger - what element initiated this
	TriggerName string // HX-Trigger-Name
	TargetID    string // HX-Target - where response will land
	Boosted     bool   // HX-Boosted - was this a boosted link/form?
}

func parseRequestContext(r *http.Request) RequestContext {
	return RequestContext{
		IsHTMX:      r.Header.Get("HX-Request") == "true",
		CurrentURL:  r.Header.Get("HX-Current-URL"),
		TriggerID:   r.Header.Get("HX-Trigger"),
		TriggerName: r.Header.Get("HX-Trigger-Name"),
		TargetID:    r.Header.Get("HX-Target"),
		Boosted:     r.Header.Get("HX-Boosted") == "true",
	}
}
