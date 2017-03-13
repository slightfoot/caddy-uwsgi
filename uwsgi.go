package uwsgi

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyfile"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"io"
	"net"
	"net/http"
	"strings"
)

type uwsgiConfig struct {
	from string
	to   string
}

type uwsgiRoundTripper struct {
	next    httpserver.Handler
	configs []uwsgiConfig
}

var headerNameReplacer = strings.NewReplacer(" ", "_", "-", "_")

func init() {
	caddy.RegisterPlugin("uwsgi", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
	http.DefaultTransport.(*http.Transport).
		RegisterProtocol("uwsgi", &uwsgiRoundTripper{})
}

func setup(c *caddy.Controller) error {

	configs, err := parseConfigs(c.Dispenser)
	if err != nil {
		return err
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		return uwsgiRoundTripper{next: next, configs: configs}
	})

	return nil
}

func parseConfigs(c caddyfile.Dispenser) ([]uwsgiConfig, error) {

	var configs []uwsgiConfig
	for c.Next() {
		config := uwsgiConfig{from: "", to: ""}

		if !c.Args(&config.from) {
			return configs, c.ArgErr()
		}

		if !c.Args(&config.to) {
			return configs, c.ArgErr()
		}

		if len(config.to) == 0 {
			return configs, c.ArgErr()
		}

		configs = append(configs, config)
	}

	return configs, nil
}

func (wrt uwsgiRoundTripper) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {

	config := wrt.match(r)
	if config == nil {
		return wrt.next.ServeHTTP(w, r)
	}

	r.URL.Host = config.to

	res, err := wrt.RoundTrip(r)
	if err != nil {
		return http.StatusBadGateway, err
	}

	dst := w.Header()
	for k, vv := range res.Header {
		if _, ok := dst[k]; ok {
			dst.Del(k)
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
	res.Body.Close()

	return res.StatusCode, nil
}

func (wrt uwsgiRoundTripper) match(r *http.Request) *uwsgiConfig {

	var c *uwsgiConfig
	var longestMatch int

	for _, config := range wrt.configs {
		basePath := config.from
		if !httpserver.Path(r.URL.Path).Matches(basePath) {
			continue
		}
		if len(basePath) > longestMatch {
			longestMatch = len(basePath)
			c = &config
		}
	}

	return c
}

func (wrt uwsgiRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	fd, err := net.Dial("tcp", req.URL.Host)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	method := req.Method
	if method == "" {
		method = "GET"
	}

	env := map[string]string{
		"REQUEST_METHOD":  method,
		"SERVER_PROTOCOL": req.Proto,
		"REQUEST_URI":     req.URL.RequestURI(),
		"QUERY_STRING":    req.URL.RawQuery,
		"HTTP_HOST":       req.Host,
		"REMOTE_ADDR":     req.RemoteAddr,
	}
	if req.TLS != nil {
		env["HTTPS"] = "on"
	}
	for key, value := range req.Header {
		header := strings.ToUpper(key)
		header = headerNameReplacer.Replace(header)
		env["HTTP_"+header] = strings.Join(value, ", ")
	}

	var buffer bytes.Buffer
	for key, value := range env {
		_key := []byte(key)
		binary.Write(&buffer, binary.LittleEndian, uint16(len(_key)))
		buffer.Write(_key)

		_value := []byte(value)
		binary.Write(&buffer, binary.LittleEndian, uint16(len(_value)))
		buffer.Write(_value)
	}

	var header [4]byte
	binary.LittleEndian.PutUint16(header[1:3], uint16(buffer.Len()))

	fd.Write(header[:])
	io.Copy(fd, &buffer)
	if req.Body != nil {
		io.Copy(fd, req.Body)
		req.Body.Close()
	}

	return http.ReadResponse(bufio.NewReader(fd), req)
}
