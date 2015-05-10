package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/afex/hystrix-go/hystrix"
)

const DefaultPort = ":3000"
const EnvPort = "PORT"

func main() {
	os.Exit(realMain())
}

func realMain() int {

	// Send all output to stdout
	log.SetOutput(os.Stdout)

	// Set port to listen must be provided from env var
	port := DefaultPort
	if os.Getenv(EnvPort) != "" {
		port = ":" + os.Getenv(EnvPort)
	}

	// Check executable path
	path, err := exec.LookPath("gox")
	if err != nil {
		log.Printf("[FATAL] executable gox is not found in PATH")
		return 1
	}
	log.Printf("[INFO] gox is in %s", path)

	// Set HandleFuncs
	http.HandleFunc("/", logWrapper(HandleCrossCompile))

	log.Printf("[INFO] start server on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Printf("[FATAL] fail to start server: %s", err.Error())
		return 1
	}

	return 0
}

// logWrapper is Handler wrapper function for logging
func logWrapper(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO] %s %s %s%s", r.UserAgent(), r.Method, r.URL.Host, r.URL.Path)
		fn(w, r)
	}
}

func HandleCrossCompile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Printf("[INFO] invalid method: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		msg := fmt.Sprintf("Invalid method: %s\n", r.Method)
		w.Write([]byte(msg))
		return
	}

	// Handle root request, show project page
	if r.URL.Path == "/" {
		http.Redirect(w, r, "https://github.com/tcnksm/gox-server", 301)
		return
	}

	// Check correct request comes
	repoComponent := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(repoComponent) != 2 {
		log.Printf("[INFO] faild to parse as repository name: %s", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request: request must be https://gox-server.herokuapp.com/USER/REPO format\n"))
		return
	}

	// Detect platform from user agent
	targetOS, targetArch := guessPlatform(r.UserAgent())

	// Set hystrix configuration
	hystrix.ConfigureCommand("gox", goxHystrixConfig)

	// Run
	resultCh := make(chan string, 1)
	errCh := hystrix.Go("gox", func() error {

		// Get source code from github
		if err := goGet(repoComponent[0], repoComponent[1]); err != nil {
			return nil
		}

		// Run gox and generate binary
		output, err := gox(repoComponent[0], repoComponent[1], targetOS, targetArch)
		if err != nil {
			return nil
		}

		resultCh <- output
		return nil
	}, nil)

	select {

	case output := <-resultCh:
		log.Printf("[INFO] cross compile is done: %s", output)
		w.WriteHeader(http.StatusOK)
		http.ServeFile(w, r, output)

	case err := <-errCh:
		log.Printf("[ERROR] failed to cross compiling: %s", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		switch err {
		case hystrix.ErrTimeout:
			w.Write([]byte("Timeout: gox-server can't handle build which takes more than 120s.\n"))
		case hystrix.ErrMaxConcurrency:
			w.Write([]byte("Too many access: gox-server can't handle more than 100 requests at one time.\n"))
		case hystrix.ErrCircuitOpen:
			w.Write([]byte("Too many errors: gox-server is unavailable now because of too many errors.\n"))
		default:
			msg := fmt.Sprintf("Build failed: %s\n", err.Error())
			w.Write([]byte(msg))
		}
	}
}

// goGet executes `go get`, currently only support github.com
func goGet(owner, repo string) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	url := fmt.Sprintf("github.com/%s/%s", owner, repo)
	cmd := exec.Command("go", "get", "-u", "-d", "-v", url)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[INFO] start to go get from %s", url)
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("[INFO] failed to go get: %s", err.Error())
		log.Printf("[INFO] STDERR of go get: %s", stderr.String())
		return err
	}
	log.Printf("[INFO] STDOUT of go get: %s", stdout.String())

	return nil
}

// gox runs gox and return output artifact path
func gox(owner, repo, targetOS, targetArch string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Change directory to project root
	project := filepath.Join(os.Getenv("GOPATH"), "src", "github.com", owner, repo)
	if err := os.Chdir(project); err != nil {
		return "", err
	}

	// Generate unique build ID from time
	// This is temp solution for avoid output overwrap
	buildID := fmt.Sprintf("%x", md5.Sum([]byte(time.Now().String())))

	output := filepath.Join("/app/builds", buildID, fmt.Sprintf("%s_%s_%s", repo, targetOS, targetArch))
	args := []string{"-os", targetOS, "-arch", targetArch, "-output", output}
	cmd := exec.Command("gox", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[INFO] start to run gox")
	if err := cmd.Start(); err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("[INFO] failed to gox: %s", err.Error())
		log.Printf("[INFO] STDERR of gox: %s", stderr.String())
		return "", err
	}

	log.Printf("[INFO] STDOUT of gox: %s", stdout.String())

	return output, nil
}

// Hystrix configuration for gox.
// See more on https://github.com/afex/hystrix-go
var goxHystrixConfig = hystrix.CommandConfig{
	// How long to wait for command to complete, in milliseconds
	Timeout: 120000,

	// MaxConcurrent is how many commands of the same type
	// can run at the same time
	MaxConcurrentRequests: 100,

	// VolumeThreshold is the minimum number of requests
	// needed before a circuit can be tripped due to health
	RequestVolumeThreshold: 1000,

	// SleepWindow is how long, in milliseconds,
	// to wait after a circuit opens before testing for recovery
	SleepWindow: 1000,

	// ErrorPercentThreshold causes circuits to open once
	// the rolling measure of errors exceeds this percent of requests
	ErrorPercentThreshold: 50,
}
