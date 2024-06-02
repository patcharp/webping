package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Request struct {
	Url            string
	IpAddr         string
	Port           int
	Method         string
	Body           []byte
	Headers        map[string]string
	Timeout        time.Duration
	SkipVerify     bool
	DnsIp          string
	ExpectedStatus int
}

type Response struct {
	RemoteAddr string
	Status     int
	body       []byte
	Headers    map[string]string
}

func main() {
	target := flag.String("target", "", "Target Url")
	method := flag.String("method", "GET", "HTTP method")
	skipVerify := flag.Bool("skip-verify", false, "Skip SSL certificate verification")
	timeOut := flag.Duration("timeout", time.Second*2, "Timeout")
	ipAddr := flag.String("ip", "", "IP address")
	expectedStatus := flag.Int("status", 200, "Expected status code")
	dnsIp := flag.String("dns-ip", "1.1.1.1", "DNS IP address")
	flag.Parse()

	req := Request{
		Url:            *target,
		IpAddr:         *ipAddr,
		Port:           0,
		Method:         strings.ToUpper(*method),
		Body:           nil,
		Headers:        nil,
		Timeout:        *timeOut,
		SkipVerify:     *skipVerify,
		DnsIp:          *dnsIp,
		ExpectedStatus: *expectedStatus,
	}
	probe(&req)
}

func probe(target *Request) {
	if !strings.HasPrefix(target.Url, "http") {
		target.Url = "http://" + target.Url
	}
	u, err := url.Parse(target.Url)
	if err != nil {
		fmt.Println("Invalid target url:", target.Url)
		fmt.Println("Usage: webping --target=https://<target>")
		return
	}
	target.Port, _ = strconv.Atoi(u.Port())
	if target.Port == 0 {
		switch u.Scheme {
		case "http":
			target.Port = 80
			break
		case "https":
			target.Port = 443
			break
		default:
			fmt.Println("Invalid target port:", target.Port)
			return
		}
	}
	if target.IpAddr == "" {
		target.IpAddr = resolve(u, target.DnsIp)
		if target.IpAddr == "" {
			fmt.Println("Invalid target address:", target.Url)
			fmt.Println("ERR: Try to set new DNS IP or set valid target url")
			return
		}
	}
	fmt.Println("Start probe url:", target.Url, "port:", target.Port, "method:", target.Method, "timeout:", target.Timeout)
	for {
		start := time.Now()
		result := Response{}
		if target.IpAddr != "" {
			result.RemoteAddr = target.IpAddr
		}
		r, err := http.NewRequest(target.Method, target.Url, bytes.NewBuffer(target.Body))
		if err != nil {
			display(target, &result, err, time.Since(start))
			time.Sleep(time.Second - time.Since(start))
			continue
		}
		ct := &httptrace.ClientTrace{
			GotConn: func(connInfo httptrace.GotConnInfo) {
				result.RemoteAddr = connInfo.Conn.RemoteAddr().String()
			},
		}
		r = r.WithContext(httptrace.WithClientTrace(r.Context(), ct))
		for k, v := range target.Headers {
			r.Header.Add(k, v)
		}
		tlsCfg := &tls.Config{}
		if target.SkipVerify {
			tlsCfg.InsecureSkipVerify = target.SkipVerify
		}
		httpTransport := &http.Transport{TLSClientConfig: tlsCfg}
		dialer := &net.Dialer{
			Timeout:   target.Timeout,
			KeepAlive: target.Timeout,
		}
		httpTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			addr = fmt.Sprintf("%s:%d", target.IpAddr, target.Port)
			return dialer.DialContext(ctx, network, addr)
		}
		httpTransport.DisableKeepAlives = true
		client := &http.Client{
			Transport: httpTransport,
			Timeout:   target.Timeout,
		}

		resp, err := client.Do(r)
		if err != nil {
			display(target, &result, err, time.Since(start))
			time.Sleep(time.Second - time.Since(start))
			continue
		}
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		result.Status = resp.StatusCode
		result.body = buf.Bytes()
		result.Headers = make(map[string]string)
		for k, v := range resp.Header {
			result.Headers[k] = v[0]
		}
		_ = resp.Body.Close()
		display(target, &result, err, time.Since(start))
		if time.Second-time.Since(start) > 0 {
			time.Sleep(time.Second - time.Since(start))
		}
	}
}

func resolve(u *url.URL, dnsIp string) string {
	if u == nil {
		return ""
	}
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second * 2,
			}
			return d.DialContext(ctx, network, fmt.Sprintf("%s:53", dnsIp))
		},
	}
	ip, _ := r.LookupHost(context.Background(), u.Hostname())
	// ipv4 filter
	var ipv4 []string
	for _, addr := range ip {
		if !strings.Contains(addr, ":") {
			ipv4 = append(ipv4, addr)
		}
	}
	switch len(ipv4) {
	case 0:
		return ""
	case 1:
		return ipv4[0]
	default:
		n := rand.Intn(len(ipv4) - 1)
		return ipv4[n]
	}
}

func display(r *Request, resp *Response, err error, d time.Duration) {
	if err != nil {
		fmt.Printf("Remote %s:%d time=%.2fms err=%s\n", resp.RemoteAddr, r.Port, float64(d.Microseconds())/1000.0, err.Error())
		return
	}
	status := "OK"
	if resp.Status != r.ExpectedStatus {
		status = "FAIL"
	}
	fmt.Printf("Connected %s code=%d time=%.2fms - %s\n", resp.RemoteAddr, resp.Status, float64(d.Microseconds())/1000.0, status)
}
