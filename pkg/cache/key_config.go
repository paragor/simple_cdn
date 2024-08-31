package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

var keySpecDelimiter = "|"

type KeyConfig struct {
	Headers []string `yaml:"headers,omitempty"`
	Cookies []string `yaml:"cookies,omitempty"`
	Query   []string `yaml:"query,omitempty"`

	NotHeaders []string `yaml:"not_headers,omitempty"`

	AllCookies bool `yaml:"all_cookies"`
	AllQuery   bool `yaml:"all_query"`
	AllHeaders bool `yaml:"all_headers"`

	notHeadersMap map[string]struct{}
	headersMap    map[string]struct{}
	cookiesMap    map[string]struct{}
	queryMap      map[string]struct{}
	m             sync.Mutex
	compiled      atomic.Bool
}

func (kc *KeyConfig) Validate() error {
	if len(kc.Headers) > 0 && kc.AllHeaders {
		return fmt.Errorf("only on of two field must be specified: headers, all_headers")
	}
	if len(kc.Cookies) > 0 && kc.AllCookies {
		return fmt.Errorf("only on of two field must be specified: cookies, all_cookies")
	}
	if len(kc.Query) > 0 && kc.AllQuery {
		return fmt.Errorf("only on of two field must be specified: query, all_query")
	}
	return nil
}

func (kc *KeyConfig) compile() {
	if kc.compiled.Load() {
		return
	}
	kc.m.Lock()
	defer kc.m.Unlock()
	if kc.compiled.Load() {
		return
	}

	kc.headersMap = make(map[string]struct{})
	kc.cookiesMap = make(map[string]struct{})
	kc.queryMap = make(map[string]struct{})
	kc.notHeadersMap = make(map[string]struct{})

	for _, k := range kc.NotHeaders {
		kc.notHeadersMap[strings.ToLower(k)] = struct{}{}
	}
	if !kc.AllHeaders {
		for _, k := range kc.Headers {
			kc.headersMap[strings.ToLower(k)] = struct{}{}
		}
	}
	if !kc.AllCookies {
		for _, k := range kc.Cookies {
			kc.cookiesMap[k] = struct{}{}
		}
	}
	if !kc.AllQuery {
		for _, k := range kc.Query {
			kc.queryMap[k] = struct{}{}
		}
	}
	kc.compiled.Store(true)
}

func sortedKeys[T any](hashtable map[string]T) []string {
	keys := make([]string, 0, len(hashtable))
	for k := range hashtable {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (kc *KeyConfig) generateRawKeyForHash(r *http.Request) string {
	kc.compile()
	key := &strings.Builder{}
	key.Grow(512)
	key.WriteString("headers" + keySpecDelimiter)
	if len(kc.Headers) > 0 || kc.AllHeaders {
		headersMap := make(map[string]string, len(r.Header))
		for k := range r.Header {
			_, inAllowList := kc.headersMap[strings.ToLower(k)]
			_, inBlackList := kc.notHeadersMap[strings.ToLower(k)]
			if !inBlackList && !inAllowList && !(kc.AllHeaders && !isBlacklistHeader(k)) {
				continue
			}
			headersMap[k] = strings.Join(r.Header[k], keySpecDelimiter) // not sortable :(
		}
		kc.addMapToKey(key, headersMap)
	}
	key.WriteString(keySpecDelimiter + "query" + keySpecDelimiter)
	if len(kc.queryMap) > 0 || kc.AllQuery {
		query := r.URL.Query()
		queryMap := make(map[string]string, len(query))
		for k := range query {
			if _, exists := kc.queryMap[k]; kc.AllQuery || exists {
				queryMap[k] = strings.Join(query[k], keySpecDelimiter) // not sortable :(
			}
		}
		kc.addMapToKey(key, queryMap)
	}
	key.WriteString(keySpecDelimiter + "cookies" + keySpecDelimiter)
	if len(kc.cookiesMap) > 0 || kc.AllCookies {
		cookies := r.Cookies()
		cookiesMap := make(map[string]string, len(cookies))
		for _, cookie := range cookies {
			if err := cookie.Valid(); err != nil {
				continue
			}
			if _, exists := kc.cookiesMap[cookie.Name]; kc.AllCookies || exists {
				cookiesMap[cookie.Name] = cookie.Value
			}
		}
		kc.addMapToKey(key, cookiesMap)
	}
	return key.String()
}

func (kc *KeyConfig) addMapToKey(key *strings.Builder, addMap map[string]string) {
	lenMap := len(addMap)
	for i, k := range sortedKeys(addMap) {
		key.WriteString(k + "=" + addMap[k])
		if i != lenMap-1 {
			key.WriteString(keySpecDelimiter)
		}
	}
}

func (kc *KeyConfig) Apply(r *http.Request) string {
	return r.URL.Path + "|" + getMD5Hash(kc.generateRawKeyForHash(r))
}

var notCachableHttpHeadersSource = map[string][]string{
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers
	"caching": {"age", "cache-control", "clear-site-data", "expires", "no-vary-search"},
	"conditionals": {
		"last-modified", "etag", "if-match", "if-none-match",
		"if-modified-since", "if-unmodified-since", "vary",
	},
	"connection management": {"connection", "keep-alive"},
	"content negotiation":   {"accept-encoding"},
	"controls":              {"max-forwards"},
	"proxies":               {"forwarded", "via"},
	"other":                 {"upgrade"},

	//https://en.wikipedia.org/wiki/list_of_http_header_fields
	"common non-standard request fields": {
		"x-requested-with", "x-forwarded-for", "x-forwarded-host",
		"x-forwarded-proto", "proxy-connection", "x-csrf-token",
		"x-request-id", "x-correlation-id", "correlation-id",
		"x-forwarded-port", "x-forwarded-proto", "x-forwarded-scheme",
		"save-data", "x-real-ip", "sec-ch-ua", "sec-ch-ua-platform",
		"dnt", "upgrade-insecure-requests", "sec-fetch-site",
		"sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest",
		"accept-language", "priority",
	},
	"my": {"cookie"},
}

var notCachableHttpHeaders = map[string]struct{}{}

func init() {
	headersList := []string{}
	for _, headers := range notCachableHttpHeadersSource {
		for _, header := range headers {
			headersList = append(headersList, header)
		}
	}
	sort.Strings(headersList)
	notCachableHttpHeaders = make(map[string]struct{}, len(headersList))
	for _, header := range headersList {
		notCachableHttpHeaders[header] = struct{}{}
	}
}

func isBlacklistHeader(key string) bool {
	_, isBlacklisted := notCachableHttpHeaders[strings.ToLower(key)]
	return isBlacklisted
}

var md5pool = sync.Pool{New: func() any { return md5.New() }}

func getMD5Hash(text string) string {
	hasher := md5pool.Get().(hash.Hash)
	if hasher == nil {
		hasher = md5.New()
	}
	defer func() {
		hasher.Reset()
		md5pool.Put(hasher)
	}()
	hasher.Reset()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
