package user

import (
	"net/http"
	"net/url"
	"testing"
)

func Test_queryCount_IsUser(t *testing.T) {
	type fields struct {
		gte int
		lte int
	}
	type args struct {
		query url.Values
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "[1,10] and 0",
			fields: fields{
				gte: 1,
				lte: 10,
			},
			args: args{
				query: map[string][]string{},
			},
			want: false,
		},
		{
			name: "[0,1] and 1",
			fields: fields{
				gte: 0,
				lte: 1,
			},
			args: args{
				query: map[string][]string{"one": {"1", "2"}},
			},
			want: true,
		},
		{
			name: "[0,1] and 0",
			fields: fields{
				gte: 0,
				lte: 1,
			},
			args: args{
				query: map[string][]string{},
			},
			want: true,
		},
		{
			name: "[0,1] and 2",
			fields: fields{
				gte: 0,
				lte: 1,
			},
			args: args{
				query: map[string][]string{"one": {"1", "2"}, "two": {"3"}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &queryCount{
				gte: tt.fields.gte,
				lte: tt.fields.lte,
			}
			rawQuery := tt.args.query.Encode()
			r, err := http.NewRequest("GET", "http://127.0.0.1?"+rawQuery, nil)
			if err != nil {
				t.Fatal("cant create request", err.Error())
			}
			if got := u.IsUser(r); got != tt.want {
				t.Errorf("IsUser() = %v, want %v", got, tt.want)
			}
		})
	}
}
