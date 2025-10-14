package util

import (
	"encoding/json"
	"log"
	"net/http"
)

type HandshakeError struct {
	Message string `json:"message"`
}

func PreventHandshakeOnError(msg string, w http.ResponseWriter) {
	error := HandshakeError{
		Message: msg,
	}

	parsedJson, err := json.Marshal(error)
	if err != nil {
		log.Panicf("Error while parsing JSON: %s", err)
	}

	w.WriteHeader(500)
	w.Write(parsedJson)
}

func FailOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}
