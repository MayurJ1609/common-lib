package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunRequiresHTTPServer(t *testing.T) {
	if err := (Server{}).Run(context.Background()); err == nil {
		t.Fatal("Run() accepted a nil HTTP server")
	}
}

func TestRunReportsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	defer listener.Close()

	err = (Server{HTTP: &http.Server{Addr: listener.Addr().String()}}).Run(context.Background())
	if err == nil {
		t.Fatal("Run() succeeded on an occupied address")
	}
}

func TestRunReturnsAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := (Server{
		HTTP:            &http.Server{Addr: "127.0.0.1:0"},
		ShutdownTimeout: time.Second,
	}).Run(ctx)
	if err != nil {
		t.Fatalf("Run() after cancellation error = %v", err)
	}
}

func TestRunReturnsWhenServerIsAlreadyClosed(t *testing.T) {
	httpServer := &http.Server{Addr: "127.0.0.1:0"}
	if err := httpServer.Close(); err != nil {
		t.Fatalf("close HTTP server: %v", err)
	}
	if err := (Server{HTTP: httpServer}).Run(context.Background()); err != nil {
		t.Fatalf("Run() on closed HTTP server error = %v", err)
	}
}

func TestRunForcesCloseAfterShutdownTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release listener address: %v", err)
	}
	httpServer := &http.Server{
		Addr: address,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			w.WriteHeader(http.StatusOK)
		}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- (Server{HTTP: httpServer, ShutdownTimeout: time.Millisecond}).Run(ctx)
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	requestResult := make(chan error, 1)
	deadline := time.Now().Add(time.Second)
	handlerStarted := false
	for time.Now().Before(deadline) {
		attemptResult := make(chan error, 1)
		go func() {
			response, err := client.Get("http://" + address)
			if response != nil {
				response.Body.Close()
			}
			attemptResult <- err
		}()
		select {
		case <-started:
			handlerStarted = true
			go func() { requestResult <- <-attemptResult }()
		case <-attemptResult:
		}
		if handlerStarted {
			break
		}
	}
	if !handlerStarted {
		close(release)
		cancel()
		t.Fatal("handler did not start")
	}

	cancel()
	select {
	case err := <-runResult:
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			close(release)
			t.Fatalf("Run() error = %v, want shutdown deadline error", err)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("Run() did not return after shutdown timeout")
	}
	close(release)
	select {
	case <-requestResult:
	case <-time.After(time.Second):
		t.Fatal("HTTP request did not complete after releasing the handler")
	}
}
