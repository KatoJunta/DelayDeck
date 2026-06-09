package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/api"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/config"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/ingest"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, err := config.ParseFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "delaydeck-relay: %v\n", err)
		return 2
	}

	machine := state.NewMachine(cfg.BufferCapacityBytes, cfg.TransitionDelay)
	server := api.NewServer(machine, cfg.SessionToken)

	if err := machine.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "delaydeck-relay: start state machine: %v\n", err)
		return 1
	}
	if err := machine.MarkReady(); err != nil {
		fmt.Fprintf(os.Stderr, "delaydeck-relay: mark ready: %v\n", err)
		return 1
	}

	ingestListener, err := ingest.StartMockListener(cfg.IngestListenAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "delaydeck-relay: ingest listener: %v\n", err)
		return 1
	}
	defer ingestListener.Close()

	if cfg.MockAutoConnect {
		go mockConnect(machine)
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("delaydeck-relay listening on %s (mock mode, ingest %s)",
			cfg.ListenAddress, ingestListener.Address())
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "delaydeck-relay: server error: %v\n", err)
		return 1
	case <-sigCh:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "delaydeck-relay: shutdown error: %v\n", err)
		return 1
	}
	return 0
}

func mockConnect(machine *state.Machine) {
	time.Sleep(10 * time.Millisecond)
	if err := machine.MockConnectInput(); err != nil {
		log.Printf("mock connect input failed: %v", err)
		return
	}
	if err := machine.MockConnectOutput(); err != nil {
		log.Printf("mock connect output failed: %v", err)
		return
	}
}
