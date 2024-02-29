package rest_api

import (
	"bjqian/signalr/signalr_server"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

type SignalRRestApiClient struct {
	httpClient *http.Client
	accessKey  string
	host       string
	hub        string
}

type Payload struct {
	Target    string `json:"target"`
	Arguments []any  `json:"arguments"`
}

func NewSignalRRestApiClient(accessKey string, hub string) (*SignalRRestApiClient, error) {
	host, accessKey, err := parseConnectionString(accessKey)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{}
	signalRRestApiClient := &SignalRRestApiClient{httpClient: httpClient, accessKey: accessKey, host: host, hub: hub}
	return signalRRestApiClient, nil
}

func (c *SignalRRestApiClient) call(pathSuffix string, method string, body any) (*http.Response, error) {
	req := &http.Request{}
	req.Method = method
	u := &url.URL{
		Scheme: "https",
		Host:   c.host,
		Path:   "/api/v1/hubs/" + c.hub + pathSuffix,
	}
	req.URL = u
	token, err := c.sign(u.String())
	if err != nil {
		return nil, err
	}
	req.Header = make(http.Header)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			log.Fatal(err)
		}
		req.Body = io.NopCloser(bytes.NewBuffer(jsonData))
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		signalr_server.LogError("Failed to make request: ", err)
	}
	signalr_server.LogDebug(fmt.Sprintf("status code: %d", res.StatusCode))
	content, _ := io.ReadAll(res.Body)
	signalr_server.LogDebug(fmt.Sprintf("response: %s", content))
	return res, err
}

func parseConnectionString(connectionString string) (string, string, error) {
	pattern := `Endpoint=https://([^;]+);AccessKey=([^;]+);`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", "", fmt.Errorf("failed to compile regex: %v", err)
	}

	matches := re.FindStringSubmatch(connectionString)
	if matches == nil || len(matches) < 3 {
		return "", "", fmt.Errorf("failed to parse connection string")
	}

	return matches[1], matches[2], nil
}

func (c *SignalRRestApiClient) sign(u string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud": strings.Split(u, "?")[0],
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	key := []byte(c.accessKey)
	return token.SignedString(key)
}

func (c *SignalRRestApiClient) RemoveUserFromGroup(user string, group string) error {
	pathSuffix := "/groups/" + group + "/users/" + user
	_, err := c.call(pathSuffix, "DELETE", nil)
	return err
}

func (c *SignalRRestApiClient) BroadCastMessage(target string, arguments ...any) error {
	pathSuffix := ""
	message := Payload{Target: target, Arguments: arguments}
	_, err := c.call(pathSuffix, "POST", message)
	return err
}
