package httpmiddleware

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const trustedProxyKey = "http.trusted_proxy"

// ProxyPolicy trusts forwarded origin headers only from explicitly configured peers.
// Configure Gin with the same list so ClientIP and browser origin agree.
func ProxyPolicy(peers []string) (gin.HandlerFunc, error) {
	prefixes := make([]netip.Prefix, 0, len(peers))
	for _, peer := range peers {
		prefix, err := netip.ParsePrefix(peer)
		if err != nil {
			addr, e := netip.ParseAddr(peer)
			if e != nil {
				return nil, fmt.Errorf("HTTP_TRUSTED_PROXIES must contain IP addresses or CIDRs")
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		prefixes = append(prefixes, prefix)
	}
	return func(c *gin.Context) {
		host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		addr, _ := netip.ParseAddr(host)
		for _, prefix := range prefixes {
			if prefix.Contains(addr.Unmap()) {
				c.Set(trustedProxyKey, true)
				break
			}
		}
		c.Next()
	}, nil
}

func RequestOrigin(c *gin.Context) (scheme, host string) {
	scheme, host = "http", c.Request.Host
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if c.GetBool(trustedProxyKey) {
		if proto := c.GetHeader("X-Forwarded-Proto"); proto == "https" || proto == "http" {
			scheme = proto
		}
		if forwarded := c.GetHeader("X-Forwarded-Host"); forwarded != "" {
			host = forwarded
		}
	}
	return
}

type AuthLimits struct {
	Login, Register, Refresh, MaxKeys int
	Window                            time.Duration
}

func DefaultAuthLimits() AuthLimits {
	return AuthLimits{Login: 10, Register: 5, Refresh: 60, MaxKeys: 10000, Window: time.Minute}
}

type authBucket struct {
	count   int
	expires time.Time
}
type authLimiter struct {
	mu          sync.Mutex
	buckets     map[string]authBucket
	nextCleanup time.Time
	options     AuthLimits
}

func (l *authLimiter) allow(key string, limit int, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !now.Before(l.nextCleanup) {
		for k, b := range l.buckets {
			if !now.Before(b.expires) {
				delete(l.buckets, k)
			}
		}
		l.nextCleanup = now.Add(l.options.Window)
	}
	b, exists := l.buckets[key]
	if !exists || !now.Before(b.expires) {
		// Saturation rejects new keys instead of evicting an active brute-force limit.
		if !exists && len(l.buckets) >= l.options.MaxKeys {
			return false, l.nextCleanup.Sub(now)
		}
		b = authBucket{expires: now.Add(l.options.Window)}
	}
	if b.count >= limit {
		return false, b.expires.Sub(now)
	}
	b.count++
	l.buckets[key] = b
	return true, 0
}

func AuthRateLimit(options AuthLimits) gin.HandlerFunc {
	defaults := DefaultAuthLimits()
	if options.Login <= 0 {
		options.Login = defaults.Login
	}
	if options.Register <= 0 {
		options.Register = defaults.Register
	}
	if options.Refresh <= 0 {
		options.Refresh = defaults.Refresh
	}
	if options.MaxKeys <= 0 {
		options.MaxKeys = defaults.MaxKeys
	}
	if options.Window <= 0 {
		options.Window = defaults.Window
	}
	l := &authLimiter{buckets: make(map[string]authBucket), options: options}
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		path := strings.TrimPrefix(c.FullPath(), "/api/v1/auth/")
		path = strings.TrimPrefix(path, "browser/")
		limit := 0
		switch path {
		case "login":
			limit = options.Login
		case "register":
			limit = options.Register
		case "refresh":
			limit = options.Refresh
		}
		if limit > 0 {
			if ok, retry := l.allow(path+":"+c.ClientIP(), limit, time.Now()); !ok {
				c.Header("Retry-After", strconv.Itoa(max(1, int(math.Ceil(retry.Seconds())))))
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
				return
			}
		}
		c.Next()
	}
}
