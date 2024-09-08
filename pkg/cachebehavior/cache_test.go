package cachebehavior

import (
	"bytes"
	"context"
	"fmt"
	"github.com/paragor/simple_cdn/pkg/cache"
	"github.com/paragor/simple_cdn/pkg/logger"
	"github.com/paragor/simple_cdn/pkg/metrics"
	"github.com/paragor/simple_cdn/pkg/user"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// newInMemoryCache only for tests
func newInMemoryCache() *inMemoryCache {
	return &inMemoryCache{data: make(map[string]*cache.Item)}
}

// inMemoryCache only for tests
type inMemoryCache struct {
	m           sync.Mutex
	data        map[string]*cache.Item
	savingCount int
	wait        sync.Cond
}

func (c *inMemoryCache) With(r *http.Request, keyConfig *cache.KeyConfig, value *cache.Item) *inMemoryCache {
	c.Set(context.Background(), keyConfig.Apply(r), value)
	return c
}

func (c *inMemoryCache) Get(_ context.Context, key string) *cache.Item {
	c.m.Lock()
	value, ok := c.data[key]
	c.m.Unlock()
	if !ok {
		return nil
	}
	return value
}

func (c *inMemoryCache) Len() int {
	c.m.Lock()
	defer c.m.Unlock()
	return len(c.data)
}
func (c *inMemoryCache) SavingCount() int {
	c.m.Lock()
	defer c.m.Unlock()
	return c.savingCount
}

func (c *inMemoryCache) Set(_ context.Context, key string, value *cache.Item) {
	c.m.Lock()
	c.savingCount++
	c.data[key] = value
	c.m.Unlock()
}

func (c *inMemoryCache) Invalidate(_ context.Context, keyPattern string) error {
	c.m.Lock()
	defer c.m.Unlock()
	re, err := regexp.Compile(strings.ReplaceAll(regexp.QuoteMeta(keyPattern), "\\*", ".*"))
	if err != nil {
		return fmt.Errorf("cant compile patter: %w", err)
	}
	for k := range c.data {
		if re.MatchString(k) {
			delete(c.data, k)
		}
	}
	return nil
}

type fakeUpstream struct {
	m       sync.Mutex
	ordered []func(*http.Request) (*http.Response, error)
	any     func(*http.Request) (*http.Response, error)
}

func newFakeUpstream() *fakeUpstream {
	return &fakeUpstream{}
}

func (u *fakeUpstream) WithOrdered(do func(*http.Request) (*http.Response, error)) *fakeUpstream {
	u.m.Lock()
	defer u.m.Unlock()
	u.ordered = append(u.ordered, do)
	return u
}
func (u *fakeUpstream) WithAny(do func(*http.Request) (*http.Response, error)) *fakeUpstream {
	u.m.Lock()
	defer u.m.Unlock()
	u.any = do
	return u
}

func (u *fakeUpstream) Do(originRequest *http.Request) (*http.Response, error) {
	do := u.any
	u.m.Lock()
	if len(u.ordered) > 0 {
		do, u.ordered = u.ordered[0], u.ordered[1:]
	}
	u.m.Unlock()
	if do == nil {
		return nil, fmt.Errorf("fake upstream have no 'any' or 'ordered' functions")
	}
	return do(originRequest)
}

func createRequest(method string, url string, headers http.Header, cookies []*http.Cookie, body []byte) *http.Request {
	var bodyReader io.Reader
	if body != nil {
		b := bytes.NewBuffer(nil)
		b.Write(body)
		bodyReader = b
	}
	request, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		panic(err)
	}
	for k, values := range headers {
		for _, v := range values {
			request.Header.Add(k, v)
		}
	}
	for _, c := range cookies {
		request.AddCookie(c)
	}
	return request
}
func createResponse(status int, headers http.Header, body []byte) *http.Response {
	response := httptest.NewRecorder()
	for k, values := range headers {
		for _, v := range values {
			response.Header().Add(k, v)
		}
	}
	response.WriteHeader(status)
	response.Write(body)
	return response.Result()
}

var once sync.Once

func initMetricsAndLogs() {
	once.Do(func() {
		logger.Init("testing", zapcore.DebugLevel)
		metrics.Init("testing")
	})
}

func Test_cacheBehavior_ServeHTTP_ProxyPass(t *testing.T) {
	initMetricsAndLogs()
	fUpstream := newFakeUpstream()
	keyConfig := &cache.KeyConfig{
		Headers:  []string{"host"},
		Cookies:  []string{},
		AllQuery: true,
	}
	fCache := newInMemoryCache()
	canPersistCache := user.Any(
		user.Not(user.HeaderExists("Authorization")),
		user.Not(user.CookieExists("token")),
	)
	canLoadCache := must1(user.UserAgentPattern(".*http.?://yandex.com/bots.*"))
	cacheControlParser := orderedCacheControlFallback{{
		user: must1(user.PathPattern("^/fallback$")),
		cacheControl: cache.CacheControl{
			Public:               true,
			MaxAge:               0,
			SMaxAge:              1 * time.Hour,
			StaleWhileRevalidate: 2 * time.Hour,
			StaleIfError:         3 * time.Hour,
		},
	}}

	cachebehavior := NewCacheBehavior(
		canPersistCache,
		canLoadCache,
		keyConfig,
		fUpstream,
		fCache,
		&cacheControlParser,
	)
	fBody := bytes.NewBuffer(nil)
	fBody.WriteString("this is body")

	testingRequest := createRequest(
		http.MethodGet,
		"http://127.0.0.1/testing?query=queryValue",
		http.Header{"test": []string{"one"}},
		nil,
		nil,
	)

	fUpstream.WithOrdered(func(request *http.Request) (*http.Response, error) {
		if err := requestIsEqualWithoutBody(testingRequest, request); err != nil {
			t.Errorf("enxpected upstream request: %s", err.Error())
		}
		return createResponse(200, http.Header{"test": []string{"one"}}, fBody.Bytes()), nil
	}).WithOrdered(func(request *http.Request) (*http.Response, error) {
		if err := requestIsEqualWithoutBody(testingRequest, request); err != nil {
			t.Errorf("enxpected upstream request: %s", err.Error())
		}
		return createResponse(400, http.Header{"test": []string{"two"}}, fBody.Bytes()), nil
	}).WithOrdered(func(request *http.Request) (*http.Response, error) {
		if err := requestIsEqualWithoutBody(testingRequest, request); err != nil {
			t.Errorf("enxpected upstream request: %s", err.Error())
		}
		return createResponse(200, http.Header{"test": []string{"one"}, "Cache-Control": []string{"public, s-maxage=60"}}, fBody.Bytes()), nil
	}).WithAny(func(request *http.Request) (*http.Response, error) {
		t.Error("unexpected call upstream")
		return nil, fmt.Errorf("unexpected call upstream")
	})

	recorder := httptest.NewRecorder()
	cachebehavior.ServeHTTP(recorder, testingRequest)
	if recorder.Code != 200 {
		t.Errorf("r1 wrong status code: expected %d, got %d", 200, recorder.Code)
	}
	expectedHeader := http.Header{}
	expectedHeader.Set("test", "one")
	expectedHeader.Set("x-cache-status", "MISS")
	if err := compareHeaders(expectedHeader, recorder.Header()); err != nil {
		t.Errorf("r1 wrong headers: %s", err.Error())
	}
	if recorder.Body.String() != fBody.String() {
		t.Errorf("r1 wrong body: expected '%s', got '%s'", fBody.String(), recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	cachebehavior.ServeHTTP(recorder, testingRequest)
	if recorder.Code != 400 {
		t.Errorf("r2 wrong status code: expected %d, got %d", 200, recorder.Code)
	}
	expectedHeader = http.Header{}
	expectedHeader.Set("test", "two")
	expectedHeader.Set("x-cache-status", "ERROR")
	if err := compareHeaders(expectedHeader, recorder.Header()); err != nil {
		t.Errorf("r2 wrong headers: %s", err.Error())
	}
	if recorder.Body.String() != fBody.String() {
		t.Errorf("r2 wrong body: expected '%s', got '%s'", fBody.String(), recorder.Body.String())
	}
	if fCache.SavingCount() != 0 {
		t.Errorf("cache saving count should %d, but have %d", 0, fCache.SavingCount())
	}
	if fCache.Len() != 0 {
		t.Errorf("cache items count should %d, but have %d", 0, fCache.Len())
	}

	recorder = httptest.NewRecorder()
	cachebehavior.ServeHTTP(recorder, testingRequest)
	if recorder.Code != 200 {
		t.Errorf("r3 wrong status code: expected %d, got %d", 200, recorder.Code)
	}
	expectedHeader = http.Header{}
	expectedHeader.Set("test", "one")
	expectedHeader.Set("x-cache-status", "MISS")
	expectedHeader.Set("cache-control", "public, s-maxage=60")
	if err := compareHeaders(expectedHeader, recorder.Header()); err != nil {
		t.Errorf("r3 wrong headers: %s", err.Error())
	}
	if recorder.Body.String() != fBody.String() {
		t.Errorf("r3 wrong body: expected '%s', got '%s'", fBody.String(), recorder.Body.String())
	}

	start := fCache.SavingCount()
	timeout := time.NewTimer(time.Second)
	for {
		time.Sleep(time.Millisecond * 100)
		if fCache.SavingCount() > start {
			break
		}
		select {
		case <-timeout.C:
			t.Fatal("no expected cache savings")
		}
	}
	testingRequestYandex := testingRequest.Clone(context.Background())
	testingRequestYandex.Header.Set("user-agent", "Yandex Bot (http://yandex.com/bots)")
	recorder = httptest.NewRecorder()
	cachebehavior.ServeHTTP(recorder, testingRequestYandex)
	if recorder.Code != 200 {
		t.Errorf("r4 wrong status code: expected %d, got %d", 200, recorder.Code)
	}
	expectedHeader = http.Header{}
	expectedHeader.Set("test", "one")
	expectedHeader.Set("x-cache-status", "HIT")
	expectedHeader.Set("cache-control", "public, s-maxage=60")
	if err := compareHeaders(expectedHeader, recorder.Header()); err != nil {
		t.Errorf("r4 wrong headers: %s", err.Error())
	}
	if recorder.Body.String() != fBody.String() {
		t.Errorf("r4 wrong body: expected '%s', got '%s'", fBody.String(), recorder.Body.String())
	}
	if fCache.SavingCount() != 1 {
		t.Errorf("cache saving count should %d, but have %d", 1, fCache.SavingCount())
	}
	if fCache.Len() != 1 {
		t.Errorf("cache items count should %d, but have %d", 1, fCache.Len())
	}
}

func requestIsEqualWithoutBody(expected *http.Request, got *http.Request) error {
	if expected.Method != got.Method {
		return fmt.Errorf("method invalid: expected %s, got %s", expected.Method, got.Method)
	}
	if expected.URL.String() != got.URL.String() {
		return fmt.Errorf("url invalid: expected %s, got %s", expected.URL.String(), got.URL.String())
	}
	return compareHeaders(expected.Header, got.Header)
}
func compareHeaders(expected, got http.Header) error {
	fmtHeader := func(header http.Header) string {
		keys := []string{}
		for k := range header {
			keys = append(keys, textproto.CanonicalMIMEHeaderKey(k))
		}
		sort.Strings(keys)
		result := []string{}
		for _, k := range keys {
			result = append(result, k+" : "+strings.Join(header.Values(k), ", "))
		}
		return strings.Join(result, "; ")
	}
	fmtExpected := fmtHeader(expected)
	fmtGot := fmtHeader(got)
	if fmtExpected != fmtGot {
		return fmt.Errorf("headers not equal: expected \n%s\ngot\n%s", fmtExpected, fmtGot)
	}
	return nil
}

func must1[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
