package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/gorilla/websocket"
)

const (
	sessionCookieName = "PHPSESSID"
)

var (
	ErrAuthenticationFailed = fmt.Errorf("authentication failed")
)

type Client struct {
	Host      string
	Username  string
	Password  string
	TLSConfig *tls.Config

	sessionID string

	http struct {
		http.Client
		*url.URL
	}

	ws struct {
		*websocket.Dialer
		*url.URL
		Origin string

		s *messageStream
	}
}

func (c *Client) ensureInit() (err error) {
	c.http.Client.Transport = &http.Transport{
		TLSClientConfig: c.TLSConfig,
	}

	c.http.URL, err = url.Parse(c.Host)
	if err != nil {
		return err
	}

	// Need a Jar
	c.http.Client.Jar, err = cookiejar.New(nil)
	return err
}

func getSessionID(cookies []*http.Cookie) string {
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			return c.Value
		}
	}

	return ""
}

func (c *Client) Login() error {
	if err := c.ensureInit(); err != nil {
		return err
	}

	// Credentials
	creds := make(url.Values, 2)
	creds.Set("username", c.Username)
	creds.Set("password", c.Password)

	resp, err := c.http.Client.PostForm(c.http.URL.String(), creds)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Stores the sessionID
	c.sessionID = getSessionID(c.http.Client.Jar.Cookies(c.http.URL))

	if c.sessionID == "" {
		return ErrAuthenticationFailed
	}

	if code := resp.StatusCode; code != http.StatusOK {
		return ErrAuthenticationFailed
	}

	return nil
}

func (c *Client) wsDial() error {
	if c.ws.s != nil {
		return nil // errors.New("cannot redial websocket")
	}

	// Websocket URL is adapted from HTTP URL
	c.ws.URL = &url.URL{
		Scheme: "wss",
		Host:   c.http.URL.Host,
		Path:   "/ws/stats",
	}

	// Origin must be same as the HTTP
	c.ws.Origin = (&url.URL{Scheme: c.http.URL.Scheme, Host: c.http.URL.Host}).String()
	h := http.Header{}
	h.Set("Origin", c.ws.Origin)

	// Dialer must have same TLS config
	wsD := &websocket.Dialer{
		EnableCompression: true,
		TLSClientConfig:   c.TLSConfig,
	}

	// Dial
	conn, _, err := wsD.Dial(c.ws.URL.String(), h)
	c.ws.s = newStream(conn)
	return err
}

func subscriptionFor(topics []string) subscriptionRequest {
	ts := make([]topic, 0, len(topics))
	for _, t := range topics {
		ts = append(ts, topic(t))
	}

	return subscriptionRequest{
		Subscribe: ts,
	}
}

func (c *Client) Subscribe(topics ...string) (<-chan interface{}, <-chan error, error) {
	if err := c.wsDial(); err != nil {
		return nil, nil, err
	}

	s := subscriptionFor(topics)
	s.SessionID = c.sessionID

	return c.ws.s.Subscribe(s)
}
