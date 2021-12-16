package webhook

import (
	"io/ioutil"
	"bytes"
	"fmt"
	"time"
    "net/http"
)

type HTTPClient struct {
    Timeout             string             `toml:"timeout"`
    Method              string             `toml:"method"`

    // HTTP Basic Auth Credentials
    Username            string             `toml:"username"`
    Password            string             `toml:"password"`

    client              *http.Client
}

func NewClient(h *HTTPClient) *HTTPClient {

    // Set default timeout
    if h.Timeout == "" {
        h.Timeout = "10s"
    }

	// Set default timeout
    if h.Method == "" {
        h.Method = "POST"
    }

    timeout, _ := time.ParseDuration(h.Timeout)

    h.client = &http.Client{
        Transport: &http.Transport{
            Proxy:           http.ProxyFromEnvironment,
        },
        Timeout: timeout,
    }

    return h
}

func (h *HTTPClient) HttpRequest(url string, data []byte) ([]byte, error) {

    req, err := http.NewRequest(h.Method, url, bytes.NewBuffer(data))
    if err != nil {
        return nil, err
    }

    if h.Username != "" || h.Password != "" {
        req.SetBasicAuth(h.Username, h.Password)
    }

    resp, err := h.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)

    if resp.StatusCode >= 300 {
        return nil, fmt.Errorf("[error] when writing to [%s] received status code: %d", url, resp.StatusCode)
    }

    if err != nil {
        return nil, fmt.Errorf("[error] when writing to [%s] received error: %v", url, err)
    }

    return body, nil
}
