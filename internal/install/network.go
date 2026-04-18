package install

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/inherelab/eget/home"
	pb "github.com/schollz/progressbar/v3"
)

func tokenFrom(value string) (string, error) {
	if strings.HasPrefix(value, "@") {
		file, err := home.Expand(value[1:])
		if err != nil {
			return "", err
		}
		body, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(body), "\r\n"), nil
	}
	return value, nil
}

var ErrNoToken = errors.New("no github token")

func getGitHubToken() (string, error) {
	if os.Getenv("EGET_GITHUB_TOKEN") != "" {
		return tokenFrom(os.Getenv("EGET_GITHUB_TOKEN"))
	}
	if os.Getenv("GITHUB_TOKEN") != "" {
		return tokenFrom(os.Getenv("GITHUB_TOKEN"))
	}
	return "", ErrNoToken
}

func setAuthHeader(req *http.Request, disableSSL bool) error {
	token, err := getGitHubToken()
	if err != nil {
		if errors.Is(err, ErrNoToken) {
			return nil
		}
		fmt.Fprintln(os.Stderr, "warning: not using github token:", err)
		return nil
	}

	if req.URL.Scheme == "https" && req.Host == "api.github.com" {
		if disableSSL {
			return fmt.Errorf("cannot use GitHub token if SSL verification is disabled")
		}
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}
	return nil
}

func Get(url string, disableSSL bool) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if err := setAuthHeader(req, disableSSL); err != nil {
		return nil, err
	}

	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: disableSSL},
	}}

	return client.Do(req)
}

func NewHTTPGetter(opts Options) HTTPGetterFunc {
	return HTTPGetterFunc(func(url string) (*http.Response, error) {
		return Get(url, opts.DisableSSL)
	})
}

type RateLimitJSON struct {
	Resources map[string]RateLimit `json:"resources"`
}

type RateLimit struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"`
}

func (r RateLimit) ResetTime() time.Time {
	return time.Unix(r.Reset, 0)
}

func GetRateLimit(opts Options) (RateLimit, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/rate_limit", nil)
	if err != nil {
		return RateLimit{}, err
	}
	if err := setAuthHeader(req, opts.DisableSSL); err != nil {
		return RateLimit{}, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return RateLimit{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RateLimit{}, err
	}

	var parsed RateLimitJSON
	if err := json.Unmarshal(body, &parsed); err != nil {
		return RateLimit{}, err
	}
	return parsed.Resources["core"], nil
}

func Download(url string, out io.Writer, getbar func(size int64) *pb.ProgressBar, opts Options) error {
	if IsLocalFile(url) {
		file, err := os.Open(url)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(out, file)
		return err
	}

	resp, err := Get(url, opts.DisableSSL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("download error: %d: %s", resp.StatusCode, body)
	}

	bar := getbar(resp.ContentLength)
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	return err
}
