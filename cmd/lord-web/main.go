// Command lord-web serves the Legend of the Red Dragon model of examples/lord-demo
// as a game in the browser. Every browser gets a runtime of its own running the
// model; the page shows the warrior the model holds and presses the keys of the
// day machine's menus. See examples/lord-demo/README.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/lordweb"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on")
	modelPath := flag.String("model", "examples/lord-demo/lord.sysml", "the LORD model to play")
	idle := flag.Duration("idle", 2*time.Hour, "how long an untouched game is kept")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "lord-web: unexpected argument %q\n", flag.Arg(0))
		os.Exit(2)
	}
	if *idle <= 0 {
		fmt.Fprintln(os.Stderr, "lord-web: -idle must be positive")
		os.Exit(2)
	}

	source, err := os.ReadFile(*modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lord-web: %v\n", err)
		os.Exit(1)
	}
	// Play a day before serving, so a model that cannot be played fails here, not
	// on the first browser.
	if _, err := lordweb.NewGame(source, 0, lordweb.Character{}); err != nil {
		fmt.Fprintf(os.Stderr, "lord-web: %s: %v\n", *modelPath, err)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lord-web: %v\n", err)
		os.Exit(1)
	}
	server := &http.Server{
		Handler:           lordweb.NewServer(source, *idle).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Printf("Legend of the Red Dragon awaits at http://%s/\n", listener.Addr())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "lord-web: %v\n", err)
		os.Exit(1)
	}
}
