package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"time"
)

// HandlerLogger is a special type for loggers per request
type HandlerLogger string

// HL is a handle logger
const HL HandlerLogger = "logger"

// GenerateRandomID generates new random ID
func GenerateRandomID(l int) (string, error) {
	b := make([]byte, l)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func logMi(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Take the context out from the request
		ctx := r.Context()

		logID, _ := GenerateRandomID(5)

		// Get the settings
		handlerLogger := log.New(os.Stdout, "["+logID+"] ", log.Lmicroseconds|log.Lshortfile)

		// Get new context with key-value "settings"
		ctx = context.WithValue(ctx, HL, handlerLogger)

		handlerLogger.Printf("%s %s\n", r.Method, r.URL)

		// Get new http.Request with the new context
		r = r.WithContext(ctx)

		// Call actuall handler
		next.ServeHTTP(w, r)

		defer func() {
			duration := time.Since(startTime)
			handlerLogger.Printf("took %s\n", duration)
		}()
	})
}
