package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/afex/hystrix-go/hystrix"
)

const DefaultPort = ":3000"
const EnvPort = "PORT"

func main() {
	os.Exit(realMain())
}
func realMain() int {

	log.SetOutput(os.Stdout)

	// Set port to listen
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

	hystrix.ConfigureCommand("gox", hystrix.CommandConfig{
		// How long to wait for command to complete, in milliseconds
		Timeout: 50000,

		// MaxConcurrent is how many commands of the same type
		// can run at the same time
		MaxConcurrentRequests: 10,

		// VolumeThreshold is the minimum number of requests
		// needed before a circuit can be tripped due to health
		RequestVolumeThreshold: 1000,

		// SleepWindow is how long, in milliseconds,
		// to wait after a circuit opens before testing for recovery
		SleepWindow: 1000,

		// ErrorPercentThreshold causes circuits to open once
		// the rolling measure of errors exceeds this percent of requests
		ErrorPercentThreshold: 50,
	})

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
		return
	}

	repoComponent := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(repoComponent) != 2 {
		log.Printf("[INFO] faild to parse as repository name: %s", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	targetOS, targetArch := guessPlatform(r.UserAgent())

	resultCh := make(chan string, 1)
	errCh := hystrix.Go("gox", func() error {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		githubURL := fmt.Sprintf("github.com/%s/%s", repoComponent[0], repoComponent[1])
		cmd := exec.Command("go", "get", "-u", "-d", githubURL)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		log.Printf("[INFO] go get from %s", githubURL)
		if err := cmd.Start(); err != nil {
			return err
		}

		if err := cmd.Wait(); err != nil {
			log.Printf("[INFO] failed to go get %s: %s", githubURL, err.Error())
			log.Printf("[INFO] STDERR of go get: %s", stderr.String())
			return err
		}
		log.Printf("[INFO] STDOUT of go get: %s", stdout.String())
		stderr.Reset()
		stdout.Reset()

		// Change directory to project root
		project := filepath.Join(os.Getenv("GOPATH"), "src", "github.com", repoComponent[0], repoComponent[1])
		if err := os.Chdir(project); err != nil {
			return err
		}

		output := filepath.Join("/app/builds", fmt.Sprintf("%s_%s_%s", repoComponent[1], targetOS, targetArch))
		args := []string{"-os", targetOS, "-arch", targetArch, "-output", output}
		cmd = exec.Command("gox", args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		log.Printf("[INFO] run gox")
		if err := cmd.Start(); err != nil {
			return err
		}

		if err := cmd.Wait(); err != nil {
			log.Printf("[INFO] failed to gox: %s", err.Error())
			log.Printf("[INFO] STDERR of gox: %s", stderr.String())
			return err
		}

		log.Printf("[INFO] STDOUT of gox: %s", stdout.String())
		stdout.Reset()
		stderr.Reset()

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
	}
}

// guessPlatform detect OS and Arch which user requests
// references https://github.com/flynn/flynn-cli-redirect
func guessPlatform(userAgent string) (string, string) {
	// Handle everything as lower case string
	userAgent = strings.ToLower(userAgent)
	return guessOS(userAgent), guessArch(userAgent)
}

func guessOS(userAgent string) string {
	if isDarwin(userAgent) {
		return "darwin"
	}

	if isWindows(userAgent) {
		return "windows"
	}

	return "linux"
}

func guessArch(userAgent string) string {
	if isAmd64(userAgent) || isDarwin(userAgent) {
		return "amd64"
	}
	return "386"
}

func isDarwin(userAgent string) bool {
	return strings.Contains(userAgent, "mac os x") || strings.Contains(userAgent, "darwin")
}

func isWindows(userAgent string) bool {
	return strings.Contains(userAgent, "windows")
}

func isAmd64(userAgent string) bool {
	return strings.Contains(userAgent, "x86_64") || strings.Contains(userAgent, "amd64") || isDarwin(userAgent)
}
