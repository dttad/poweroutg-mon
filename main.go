package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// getenvInt reads an env var and parses it as int seconds, fallback if missing/invalid
func getenvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		log.Printf("[!] Invalid env %s=%q, using %ds", key, val, fallback)
		return fallback
	}
	return n
}

// ping returns true if ping to addr is successful
func ping(ctx context.Context, addr string) (bool, error) {
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", addr)
	if err := cmd.Run(); err != nil {
		// timeout or other error means unreachable
		if errors.Is(err, context.DeadlineExceeded) {
			return false, fmt.Errorf("ping timeout: %w", err)
		}
		return false, nil
	}
	return true, nil
}

// poweroff runs systemctl poweroff
func poweroff() error {
	log.Println("[!] Poweroff triggered")
	cmd := exec.Command("systemctl", "poweroff")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// monitor checks router, powers off if unreachable for timeoutSec
func monitor(addr string, intervalSec, timeoutSec, logEverySec int) error {
	var (
		failSince  time.Time
		logSkipped int
		tick       = time.NewTicker(time.Duration(intervalSec) * time.Second)
	)
	defer tick.Stop()
	log.Printf("[*] Monitor %s every %ds, shutdown after %ds offline", addr, intervalSec, timeoutSec)
	for {
		select {
		case <-tick.C:
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(intervalSec)*time.Second)
			ok, err := ping(ctx, addr)
			cancel()
			if err != nil {
				log.Printf("[!] Ping err: %v", err)
			}
			if ok {
				log.Println("[+] Ping OK")
				failSince, logSkipped = time.Time{}, 0
			} else {
				if failSince.IsZero() {
					failSince = time.Now()
					log.Println("[!] Ping failed, timer started")
					logSkipped = 0
				} else {
					elapsed := int(time.Since(failSince).Seconds())
					if elapsed >= (logSkipped+1)*logEverySec || elapsed >= timeoutSec {
						log.Printf("[!] Ping failed for %ds/%ds", elapsed, timeoutSec)
						logSkipped++
					}
					if elapsed >= timeoutSec {
						if err := poweroff(); err != nil {
							log.Printf("[!] Poweroff err: %v", err)
						}
						return nil
					}
				}
			}
		}
	}
}

// trapSignals handles SIGINT/SIGTERM for graceful exit
func trapSignals() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sig
		log.Printf("[*] Exit by signal: %v", s)
		os.Exit(0)
	}()
}

func main() {
	// Log to stdout, short timestamp
	log.SetFlags(log.LstdFlags)
	log.SetOutput(os.Stdout)
	trapSignals()

	// Config from env, seconds for easy override
	addr := os.Getenv("TARGET_ADDR")
	if addr == "" {
		addr = "192.168.1.1"
	}
	intervalSec := getenvInt("TARGET_INTERVAL", 5)
	timeoutSec := getenvInt("TARGET_TIMEOUT", 120)
	logEverySec := getenvInt("TARGET_LOG_EVERY", 30)

	if intervalSec <= 0 || timeoutSec <= 0 || logEverySec <= 0 {
		log.Println("[!] Invalid time values, aborting")
		os.Exit(1)
	}
	if addr == "" {
		log.Println("[!] Address to monitor is empty, aborting")
		os.Exit(1)
	}

	if err := monitor(addr, intervalSec, timeoutSec, logEverySec); err != nil {
		log.Printf("[!] Monitor err: %v", err)
		os.Exit(1)
	}
}
