package handlers

import (
	"fmt"
	"net/http"
)

func ErrorResponse(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
</Error>`, code, message)
}

func ErrorResponseWithStatus(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
</Error>`, code, message)
}
