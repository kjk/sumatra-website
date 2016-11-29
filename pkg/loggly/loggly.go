package loggly

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Version is version of this library sent to loggly
const Version = "0.4.3"

const api = "https://logs-01.loggly.com/bulk/{token}"
const userAgent = "go-loggly (version: " + Version + ")"

// Message describes a message sent to loggly
type Message map[string]interface{}

var nl = []byte{'\n'}

// Client is loggly client
type Client struct {
	// Optionally output logs to the given writer.
	Writer io.Writer

	// Size of buffer before flushing [100]
	BufferSize int

	// Flush interval regardless of size [5s]
	FlushInterval time.Duration

	endpoint string

	// Token string.
	Token string

	// Default properties.
	Defaults Message

	buffer   [][]byte
	tags     []string
	tagsList string
	sync.Mutex
}

// New returns a new loggly client with the given `token`.
// Optionally pass `tags` or set them later with `.Tag()`.
func New(token string, tags ...string) *Client {
	host, err := os.Hostname()
	defaults := Message{}

	if err == nil {
		defaults["hostname"] = host
	}

	c := &Client{
		BufferSize:    100,
		FlushInterval: 5 * time.Second,
		Token:         token,
		endpoint:      strings.Replace(api, "{token}", token, 1),
		buffer:        make([][]byte, 0),
		Defaults:      defaults,
	}

	c.Tag(tags...)

	go c.start()

	return c
}

// Send buffers `msg` for async sending.
func (c *Client) Send(msg Message) error {
	if _, exists := msg["timestamp"]; !exists {
		msg["timestamp"] = time.Now().UnixNano() / int64(time.Millisecond)
	}
	merge(msg, c.Defaults)

	json, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()

	if c.Writer != nil {
		fmt.Fprintf(c.Writer, "%s\n", string(json))
	}

	c.buffer = append(c.buffer, json)

	if len(c.buffer) >= c.BufferSize {
		go c.Flush()
	}

	return nil
}

// Write raw data to loggly.
func (c *Client) Write(b []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	if c.Writer != nil {
		fmt.Fprintf(c.Writer, "%s", b)
	}

	c.buffer = append(c.buffer, b)

	if len(c.buffer) >= c.BufferSize {
		go c.Flush()
	}

	return len(b), nil
}

// Log logs key/value pairs
func (c *Client) Log(args ...interface{}) error {
	n := len(args)
	if n == 0 {
		return errors.New("Didn't provide any arguments")
	}
	if n%2 != 0 {
		return fmt.Errorf("Number of arguments is odd (%d)", n)
	}
	msg := Message{}
	for i := 0; i < n/2; i++ {
		k := args[i*2]
		v := args[i*2+1]
		ks, ok := k.(string)
		if !ok {
			return fmt.Errorf("key '%v' should be string (is of type %T)", k, k)
		}
		msg[ks] = v
	}
	return c.Send(msg)
}

// Flush the buffered messages.
func (c *Client) Flush() error {
	c.Lock()

	if len(c.buffer) == 0 {
		c.Unlock()
		return nil
	}

	body := bytes.Join(c.buffer, nl)

	c.buffer = nil
	tagsList := c.tagsList
	c.Unlock()

	client := &http.Client{}
	req, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Content-Type", "text/plain")
	req.Header.Add("Content-Length", string(len(body)))

	if tagsList != "" {
		req.Header.Add("X-Loggly-Tag", tagsList)
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	return err
}

// Tag adds the given `tags` for all logs.
func (c *Client) Tag(tags ...string) {
	c.Lock()
	defer c.Unlock()

	for _, tag := range tags {
		c.tags = append(c.tags, tag)
	}
	c.tagsList = strings.Join(c.tags, ",")
}

func (c *Client) start() {
	for {
		time.Sleep(c.FlushInterval)
		c.Flush()
	}
}

// Merge others into a.
func merge(a Message, others ...Message) {
	for _, msg := range others {
		for k, v := range msg {
			a[k] = v
		}
	}
}
