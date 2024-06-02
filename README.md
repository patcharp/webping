## Webping

Simple web ping tool. Inspire from ping command.

### Usage:
```
Usage of webping:
  -dns-ip string
        DNS IP address (default "1.1.1.1")
  -ip string
        IP address
  -method string
        HTTP method (default "GET")
  -skip-verify
        Skip SSL certificate verification
  -status int
        Expected status code (default 200)
  -target string
        Target Url
  -timeout duration
        Timeout (default 2s)
```

### Example:
```bash
# Set target with timeout
webping --target=https://github.com --timeout=4s

# Set target with specific remote ip
webping --target=https://github.com --ip=127.0.0.1

# Set target with custom dns
webping --target=https://github.com --dns-ip=8.8.8.8

# Set target with expect http status code to 404
webping --target=https://gihub.com/test --status=404
```