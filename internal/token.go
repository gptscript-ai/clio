package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/pkg/browser"
)

const (
	AppName          = "clio"
	Context          = "github.com/gptscript-ai/" + AppName + "/context"
	proxyURL         = "https://clio-proxy.gptscript.ai"
	oauthServiceName = "GitHub"
)

func enter(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := fmt.Scanln()
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func TokenAndURL(ctx context.Context, appName string) (string, string, error) {
	ctx, sigCancel := signal.NotifyContext(ctx, os.Interrupt)
	defer sigCancel()

	tokenFile, err := xdg.ConfigFile(filepath.Join(appName, "token"))
	if err != nil {
		return "", "", err
	}

	var existed bool
	tokenData, err := os.ReadFile(tokenFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", "", fmt.Errorf("reading %s: %w", tokenFile, err)
	} else if err == nil {
		existed = true
	}

	token := strings.TrimSpace(string(tokenData))
	if testToken(ctx, token) {
		return token, proxyURL + "/llm/openai", nil
	}

	uuid := uuid.NewString()
	loginURL, err := create(ctx, uuid)
	if err != nil {
		return "", "", fmt.Errorf("failed to create login request: %w", err)
	}

	if !existed {
		fmt.Println()
		fmt.Println(color.GreenString("Authentication is needed"))
		fmt.Println(color.GreenString("========================"))
		fmt.Println()
		fmt.Println(color.CyanString("GitHub") + " is used for authentication using the browser. This can be bypassed by setting")
		fmt.Println("the env var " + color.CyanString("CLIO_OPENAI_API_KEY") + " to your API key.")
		fmt.Println()
		fmt.Println(color.GreenString("Press ENTER to continue (CTRL+C to exit)"))
		if err := enter(ctx); err != nil {
			return "", "", err
		}
		fmt.Println()
	}

	fmt.Printf("Opening browser to %s. if there is an issue paste this link into a browser manually\n", loginURL)
	_ = browser.OpenURL(loginURL)

	ctx, timeoutCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer timeoutCancel()

	token, err = get(ctx, uuid)
	if err != nil {
		return "", "", fmt.Errorf("failed to get token: %w", err)
	}

	return token, proxyURL + "/llm/openai", os.WriteFile(tokenFile, []byte(token), 0600)
}

type createRequest struct {
	ServiceName string `json:"serviceName,omitempty"`
	ID          string `json:"id,omitempty"`
}

type createResponse struct {
	TokenPath string `json:"token-path,omitempty"`
}

func create(ctx context.Context, uuid string) (string, error) {
	var data bytes.Buffer
	if err := json.NewEncoder(&data).Encode(createRequest{ID: uuid, ServiceName: oauthServiceName}); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL+"/api/token-request", &data)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer req.Body.Close()

	var tokenResponse createResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}

	if tokenResponse.TokenPath == "" {
		return "", fmt.Errorf("no token found in response to %s", req.URL)
	}

	return tokenResponse.TokenPath, nil
}

type checkResponse struct {
	Error string `json:"error,omitempty"`
	Token string `json:"token,omitempty"`
}

func get(ctx context.Context, uuid string) (string, error) {
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL+"/api/token-request/"+uuid, nil)
		if err != nil {
			return "", err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		var checkResponse checkResponse
		if err := json.NewDecoder(resp.Body).Decode(&checkResponse); err != nil {
			return "", err
		}

		if checkResponse.Error != "" {
			return "", errors.New(checkResponse.Error)
		}

		if checkResponse.Token != "" {
			return checkResponse.Token, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Millisecond * 500):
		}
	}
}

func testToken(ctx context.Context, token string) bool {
	if token == "" {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "GET", proxyURL+"/api/me", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == 200
}
