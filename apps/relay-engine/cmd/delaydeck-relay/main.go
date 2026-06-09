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
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/output"
	_ "github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/rtmpcompat"
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
	server := api.NewServer(machine, cfg.SessionToken, cfg.Mode.String())

	var forwardingCoordinator *ingest.ForwardingCoordinator
	if cfg.Mode == config.RunModeForwarding {
		forwardingCoordinator = ingest.NewForwardingCoordinator()
		machine.SetEnableDelayCoordinator(forwardingCoordinator)
	}

	if err := machine.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "delaydeck-relay: start state machine: %v\n", err)
		return 1
	}
	if err := machine.MarkReady(); err != nil {
		fmt.Fprintf(os.Stderr, "delaydeck-relay: mark ready: %v\n", err)
		return 1
	}

	var ingestAddress string
	switch cfg.Mode {
	case config.RunModeForwarding:
		dest, err := output.ParseDestination(cfg.OutputURL, cfg.OutputStreamKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "delaydeck-relay: output destination: %v\n", err)
			return 1
		}

		rtmpServer, err := ingest.StartRTMPServer(cfg.IngestListenAddress, dest, machine, ingest.ForwardingOptions{
			FixedDelaySeconds:   cfg.FixedDelaySeconds,
			BufferCapacityBytes: cfg.BufferCapacityBytes,
		}, forwardingCoordinator)
		if err != nil {
			fmt.Fprintf(os.Stderr, "delaydeck-relay: ingest server: %v\n", err)
			return 1
		}
		defer rtmpServer.Close()
		ingestAddress = rtmpServer.Address()
	default:
		mockListener, err := ingest.StartMockListener(cfg.IngestListenAddress)
		if err != nil {
			fmt.Fprintf(os.Stderr, "delaydeck-relay: ingest listener: %v\n", err)
			return 1
		}
		defer mockListener.Close()
		ingestAddress = mockListener.Address()

		if cfg.MockAutoConnect {
			go mockConnect(machine)
		}
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("delaydeck-relay listening on %s (mode=%s, ingest %s, fixed_delay=%ds)",
			cfg.ListenAddress, cfg.Mode, ingestAddress, cfg.FixedDelaySeconds)
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
