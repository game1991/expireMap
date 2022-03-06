package expiremap

import (
	"testing"
	"time"
)

type data struct {
	Name string
	Code int
	Foo  *foo
}

type foo struct {
	cat bool
	dog bool
}

func Test_cache_Set(t *testing.T) {
	type args struct {
		key       string
		value     interface{}
		timestamp time.Duration
	}
	tests := []struct {
		name    string
		c       Cacher
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "测试过期的key",
			c:    GetCacher(),
			args: args{
				key: "test1",
				value: &data{
					Name: "value",
					Code: 100,
					Foo: &foo{
						cat: true,
						dog: false,
					},
				},
				timestamp: 3 * time.Second,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.c.Set(tt.args.key, tt.args.value, tt.args.timestamp); (err != nil) != tt.wantErr {
				t.Errorf("cache.Set() error = %v, wantErr %v", err, tt.wantErr)
			}

			t.Log(tt.args.key, tt.c.TTL(tt.args.key), tt.c.Size())

			data, err := tt.c.Get(tt.args.key, 0)
			if err != nil {
				t.Log(err)
				t.FailNow()
			}

			t.Log(data)

			select {
			case <-time.After(tt.args.timestamp + time.Second):
				t.Log(tt.args.key, tt.c.TTL(tt.args.key))
			}

		})
	}
}
