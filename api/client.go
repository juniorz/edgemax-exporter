package api

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

const (
	keepAliveInterval = 5 * time.Minute
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
		keepAliveDone chan interface{}
	}

	ws struct {
		*websocket.Dialer
		*url.URL
		Origin string
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

	c.keepAlive()

	return nil
}

func (c *Client) keepAlive() {
	// TODO: close if not nil?
	c.http.keepAliveDone = make(chan interface{})

	go func() {
		for {
			select {
			case <-time.After(keepAliveInterval):
				log.Printf("Renewing session...")

				heartBeatURL := c.http.URL.ResolveReference(&url.URL{
					Path:     "/api/edge/heartbeat.json",
					RawQuery: fmt.Sprintf("_=%d", time.Now().UnixNano()),
				})

				if _, err := c.http.Get(heartBeatURL.String()); err != nil {
					log.Printf("keep-alive error: %s", err)
				}

				log.Printf("Session renewed.")
			case <-c.http.keepAliveDone:
				return
			}
		}
	}()
}

func (c *Client) wsDial() (*websocket.Conn, error) {
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

	conn, _, err := wsD.Dial(c.ws.URL.String(), h)
	return conn, err
}

func (c *Client) Subscribe(topics ...string) (*Subscription, error) {
	conn, err := c.wsDial()
	if err != nil {
		return nil, err
	}

	return newSubscription(conn, c.sessionID, topics...)
}

func (c *Client) Close() error {
	close(c.http.keepAliveDone)

	return nil
}
