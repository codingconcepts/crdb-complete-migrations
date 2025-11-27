package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/codingconcepts/errhandler"
)

func Log(n errhandler.Wrap) errhandler.Wrap {
	return func(w http.ResponseWriter, r *http.Request) error {
		start := time.Now()
		defer func() {
			log.Printf("completed in %v", time.Since(start))
		}()

		log.Printf("%s %s", r.Method, r.RequestURI)

		return n(w, r)
	}
}
