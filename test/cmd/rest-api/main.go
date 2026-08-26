package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	goconf "github.com/nitsugaro/go-conf"
	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/test/restserver"
)

func main() {
	address := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	configFile := flag.String("config", "test/.config.json", "go-conf JSON file")
	journeyFolder := flag.String("journeys", "test/config/auth/journeys", "journey storage folder")
	scriptFolder := flag.String("scripts", "test/js-scripts", "script storage folder")
	schemaFolder := flag.String("schemas", "test/config/auth/schemas", "developer schema storage folder")
	scheduleFolder := flag.String("schedules", "test/config/auth/schedules", "schedule storage folder")
	cacheFolder := flag.String("cache", "test/config/auth/cache", "cache instance storage folder")
	encryptKey := flag.String("encrypt-key", "0123456789abcdef0123456789abcdef", "16, 24, or 32-byte context encryption key")
	flag.Parse()

	if err := goconf.LoadConfig(*configFile); err != nil {
		fatal(err)
	}
	env.SetEnvironment()
	router, err := restserver.New(&restserver.Config{
		JourneyFolder: *journeyFolder, ScriptFolder: *scriptFolder, SchemaFolder: *schemaFolder, ScheduleFolder: *scheduleFolder, CacheFolder: *cacheFolder, EncryptKey: []byte(*encryptKey),
	})
	if err != nil {
		fatal(err)
	}
	server := &http.Server{Addr: *address, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		fmt.Printf("journey REST API listening on http://%s\n", *address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
