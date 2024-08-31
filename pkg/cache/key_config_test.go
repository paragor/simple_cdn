package cache

import (
	"net/http"
	"testing"
	"time"
)

func TestKeyConfig_generateRawKeyForHash(t *testing.T) {
	type fields struct {
		Headers    []string
		Cookies    []string
		Query      []string
		NotHeaders []string
		AllCookies bool
		AllQuery   bool
		AllHeaders bool
	}
	type args struct {
		r *http.Request
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "all true",
			fields: fields{
				Headers:    nil,
				Cookies:    nil,
				Query:      nil,
				NotHeaders: nil,
				AllCookies: true,
				AllQuery:   true,
				AllHeaders: true,
			},
			args: args{
				r: createRequest(
					"a=1&b=2&d&c=4",
					http.Header{
						"h1": []string{"hv1"},
						"h2": []string{"hv2"},
					},
					[]http.Cookie{
						{
							Name:    "c1",
							Value:   "cv1",
							Expires: time.Now().Add(time.Hour),
						},
						{
							Name:    "c3",
							Value:   "cv3",
							Expires: time.Now().Add(time.Hour),
						},
						{
							Name:    "c1",
							Value:   "cv2",
							Expires: time.Now().Add(time.Hour),
						},
						{
							Name:    "Without time",
							Value:   "is invalid",
							Expires: time.Time{},
						},
					},
				),
			},
			want: "headers|H1=hv1|H2=hv2|query|a=1|b=2|c=4|d=|cookies|c1=cv2|c3=cv3",
		},
		{
			name: "example yandex.yaml",
			fields: fields{
				Headers:    []string{"host"},
				Cookies:    nil,
				Query:      nil,
				NotHeaders: nil,
				AllCookies: false,
				AllQuery:   true,
				AllHeaders: false,
			},
			args: args{
				r: createRequest(
					"1_three&0_one=two",
					http.Header{
						"host":    []string{"www.google.com"},
						"another": []string{"one"},
					},
					[]http.Cookie{
						{
							Name:  "c1",
							Value: "cv2",
						},
						{
							Name:  "c1",
							Value: "cv2",
						},
					},
				),
			},
			want: "headers|Host=www.google.com|query|0_one=two|1_three=|cookies|",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := &KeyConfig{
				Headers:    tt.fields.Headers,
				Cookies:    tt.fields.Cookies,
				Query:      tt.fields.Query,
				NotHeaders: tt.fields.NotHeaders,
				AllCookies: tt.fields.AllCookies,
				AllQuery:   tt.fields.AllQuery,
				AllHeaders: tt.fields.AllHeaders,
			}
			if got := kc.generateRawKeyForHash(tt.args.r); got != tt.want {
				t.Errorf("generateRawKeyForHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createRequest(query string, header http.Header, cookies []http.Cookie) *http.Request {
	request, err := http.NewRequest("GET", "http://127.0.0.1/?"+query, nil)
	if err != nil {
		panic(err)
	}
	for k, values := range header {
		for _, v := range values {
			request.Header.Add(k, v)
		}

	}
	for _, c := range cookies {
		request.AddCookie(&c)
	}
	return request
}
