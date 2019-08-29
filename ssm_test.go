package ssm

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/google/go-cmp/cmp"
)

func TestParamStore_Read(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		params  []ssm.Parameter
		config  reflect.Type
		want    []value
		wantErr bool
	}{
		{
			name: "String",
			params: []ssm.Parameter{
				stringParam("/foo", "bar"),
			},
			config: reflect.TypeOf(struct {
				Foo string `ssm:"foo"`
			}{}),
			want: []value{
				{path: "Foo", value: "bar"},
			},
		},
		{
			name: "StringList",
			params: []ssm.Parameter{
				stringListParam("/foo", "a,b,c"),
			},
			config: reflect.TypeOf(struct {
				Foo []string `ssm:"foo"`
			}{}),
			want: []value{
				{path: "Foo", value: []string{"a", "b", "c"}},
			},
		},
		{
			name: "SecureString",
			params: []ssm.Parameter{
				secureStringParam("/foo", "foo"),
			},
			config: reflect.TypeOf(struct {
				Foo string `ssm:"foo"`
			}{}),
			want: []value{
				{path: "Foo", value: "foo"},
			},
		},
		{
			name:    "OptionPrefix",
			options: []Option{WithPrefix("dev")},
			params: []ssm.Parameter{
				stringParam("/dev/foo", "abc"),
				stringParam("/prod/foo", "def"),
			},
			config: reflect.TypeOf(struct {
				Foo string `ssm:"foo"`
			}{}),
			want: []value{
				{path: "Foo", value: "abc"},
			},
		},
		{
			name:    "OptionPrefix_SlashPrefix",
			options: []Option{WithPrefix("/dev")}, // trim /
			params: []ssm.Parameter{
				stringParam("/dev/foo", "abc"),
			},
			config: reflect.TypeOf(struct {
				Foo string `ssm:"foo"`
			}{}),
			want: []value{
				{path: "Foo", value: "abc"},
			},
		},
		{
			name:    "OptionPrefix_SlashSuffix",
			options: []Option{WithPrefix("dev/")}, // trim /
			params: []ssm.Parameter{
				stringParam("/dev/foo", "abc"),
			},
			config: reflect.TypeOf(struct {
				Foo string `ssm:"foo"`
			}{}),
			want: []value{
				{path: "Foo", value: "abc"},
			},
		},
		{
			name:    "OptionTag",
			options: []Option{WithTag("config")},
			params: []ssm.Parameter{
				stringParam("/foo", "abc"),
				stringParam("/bar", "123"),
			},
			config: reflect.TypeOf(struct {
				Foo string `config:"foo"`
				Bar string `other:"bar"`
			}{}),
			want: []value{
				{path: "Foo", value: "abc"},
				// Bar was not set
			},
		},
		{
			name:    "OptionParseDuration",
			options: []Option{WithParseDuration()},
			params: []ssm.Parameter{
				stringParam("/timeout", "5s"),
				stringParam("/not_duration", "foo"),
			},
			config: reflect.TypeOf(struct {
				Timeout     time.Duration `ssm:"timeout"`
				NotDuration string        `ssm:"not_duration"`
			}{}),
			want: []value{
				{path: "Timeout", value: 5 * time.Second},
				{path: "NotDuration", value: "foo"},
			},
		},
		{
			name:    "OptionParseDurationErrInvalid",
			options: []Option{WithParseDuration()},
			params: []ssm.Parameter{
				stringParam("/timeout", "invalid duration"),
			},
			config: reflect.TypeOf(struct {
				Timeout time.Duration `ssm:"timeout"`
			}{}),
			wantErr: true,
		},
		{
			name:    "OptionParseTime",
			options: []Option{WithParseTime(time.RFC3339)},
			params: []ssm.Parameter{
				stringParam("/date", "2020-01-02T15:04:05Z"),
				stringParam("/not_time", "foo"),
			},
			config: reflect.TypeOf(struct {
				Date    time.Time `ssm:"date"`
				NotTime string    `ssm:"not_time"`
			}{}),
			want: []value{
				{path: "Date", value: time.Date(2020, 1, 2, 15, 4, 5, 0, time.UTC)},
				{path: "NotTime", value: "foo"},
			},
		},
		{
			name:    "OptionParseTimeErr",
			options: []Option{WithParseTime(time.RFC3339)},
			params: []ssm.Parameter{
				stringParam("/date", "invalid time"),
			},
			config: reflect.TypeOf(struct {
				Date time.Time `ssm:"date"`
			}{}),
			wantErr: true,
		},
		{
			name:    "OptionWithParseNumber",
			options: []Option{WithParseNumber()},
			params: []ssm.Parameter{
				stringParam("/a", "1"),
				stringParam("/b", "2"),
				stringParam("/c", "3"),
				stringParam("/d", "4"),
				stringParam("/e", "5"),
				stringParam("/f", "6.7"),
				stringParam("/g", "8.9"),
			},
			config: reflect.TypeOf(struct {
				Int     int     `ssm:"a"`
				Int8    int8    `ssm:"b"`
				Int16   int16   `ssm:"c"`
				Int32   int32   `ssm:"d"`
				Int64   int64   `ssm:"e"`
				Float32 float32 `ssm:"f"`
				Float64 float64 `ssm:"g"`
			}{}),
			want: []value{
				{path: "Int", value: int(1)},
				{path: "Int8", value: int8(2)},
				{path: "Int16", value: int16(3)},
				{path: "Int32", value: int32(4)},
				{path: "Int64", value: int64(5)},
				{path: "Float32", value: float32(6.7)},
				{path: "Float64", value: float64(8.9)},
			},
		},
		{
			name:    "OptionWithParseNumber_Slice",
			options: []Option{WithParseNumber()},
			params: []ssm.Parameter{
				stringListParam("/ints", "1,2,3"),
				stringListParam("/floats", "1.23,4.56,7.89"),
			},
			config: reflect.TypeOf(struct {
				Ints   []int     `ssm:"ints"`
				Floats []float64 `ssm:"floats"`
			}{}),
			want: []value{
				{path: "Ints", value: []int{1, 2, 3}},
				{path: "Floats", value: []float64{1.23, 4.56, 7.89}},
			},
		},
		{
			name: "SetPointer",
			params: []ssm.Parameter{
				stringParam("/foo", "bar"),
			},
			config: reflect.TypeOf(struct {
				Foo *string `ssm:"foo"`
			}{}),
			want: []value{
				{path: "Foo", value: aws.String("bar")},
			},
		},
		{
			name: "Nested",
			params: []ssm.Parameter{
				stringParam("/root", "foo"),
				stringParam("/db/user", "bar"),
				stringParam("/db/password", "baz"),
				stringParam("/ext/a", "qux"),
			},
			config: reflect.TypeOf(struct {
				Root     string `ssm:"root"`
				Database struct {
					User string `ssm:"user"`
					Pass string `ssm:"password"`
				} `ssm:"db"`
				External *struct {
					A string `ssm:"a"`
				} `ssm:"ext"`
			}{}),
			want: []value{
				{path: "Root", value: "foo"},
				{path: "Database.User", value: "bar"},
				{path: "Database.Pass", value: "baz"},
				{path: "External.A", value: "qux"},
			},
		},
		{
			name: "IngoreUnexported",
			params: []ssm.Parameter{
				stringParam("/foo", "foo"),
			},
			config: reflect.TypeOf(struct {
				Foo string `ssm:"foo"`
				bar string // no ssm tag
			}{}),
			want: []value{
				{path: "Foo", value: "foo"},
			},
		},
		{
			name:    "NotFound",
			options: []Option{WithPrefix("prod")},
			params: []ssm.Parameter{
				stringParam("/dev/foo", "foo"),
			},
			config: reflect.TypeOf(struct {
				Foo string `ssm:"foo"`
			}{}),
			wantErr: true,
		},
		{
			name: "ErrConvertStringToSlice",
			params: []ssm.Parameter{
				stringParam("/names", "alice"),
			},
			config: reflect.TypeOf(struct {
				Names []string `ssm:"names"`
			}{}),
			wantErr: true,
		},
		{
			name: "ErrUnexportedWithTag",
			params: []ssm.Parameter{
				stringParam("/foo", "foo"),
			},
			config: reflect.TypeOf(struct {
				value string `ssm:"foo"` // nolint: unused
			}{}),
			wantErr: true,
		},
		{
			name: "ErrUnexportedNested",
			params: []ssm.Parameter{
				stringParam("/foo/bar", "foo"),
			},
			config: reflect.TypeOf(struct {
				Foo struct {
					bar string `ssm:"bar"`
				} `ssm:"foo"`
			}{}),
			wantErr: true,
		},
		{
			name: "ErrNotSupportedInt",
			params: []ssm.Parameter{
				stringParam("/number", "123"),
			},
			config: reflect.TypeOf(struct {
				Number int `ssm:"number"` // Not allowed without WithParseNumber
			}{}),
			wantErr: true,
		},
		{
			name: "ErrStringListToString",
			params: []ssm.Parameter{
				stringListParam("/names", "alice,bob"),
			},
			config: reflect.TypeOf(struct {
				Names string `ssm:"names"` // Not allowed to set list
			}{}),
			wantErr: true,
		},
		{
			name:    "ErrParseInt",
			options: []Option{WithParseNumber()},
			params: []ssm.Parameter{
				stringParam("/name", "alice"),
			},
			config: reflect.TypeOf(struct {
				Name int `ssm:"name"` // Cannot parse "alice" as int
			}{}),
			wantErr: true,
		},
		{
			name:    "ErrParseFloat",
			options: []Option{WithParseNumber()},
			params: []ssm.Parameter{
				stringParam("/name", "alice"),
			},
			config: reflect.TypeOf(struct {
				Name float64 `ssm:"name"` // Cannot parse "alice" as float
			}{}),
			wantErr: true,
		},
		{
			name: "ErrParseIntSlice",
			params: []ssm.Parameter{
				stringListParam("/names", "alice,bob"),
			},
			config: reflect.TypeOf(struct {
				Names []int `ssm:"names"` // Cannot parse "alice" as int
			}{}),
			wantErr: true,
		},
		{
			name: "ErrEncryptedSlice",
			params: []ssm.Parameter{
				secureStringParam("/names", "alice"),
			},
			config: reflect.TypeOf(struct {
				Names []string `ssm:"names"`
			}{}),
			wantErr: true,
		},
		{
			name: "ErrUnsupported",
			params: []ssm.Parameter{
				stringParam("/foo", "bar"),
			},
			config: reflect.TypeOf(struct {
				Chan chan string `ssm:"foo"`
			}{}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSSM{params: tt.params}
			ps, err := NewParamStore(
				append(tt.options, WithClient(mock))...,
			)
			if err != nil {
				t.Fatal(err)
			}

			val := reflect.New(tt.config)
			got := val.Interface()
			err = ps.Read(context.Background(), got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Read() err = %v, want err = %t", err, tt.wantErr)
			}
			if tt.wantErr {
				t.Logf("Got expected error: %v", err)
			}
			check(t, val.Elem().Interface(), tt.want)
		})
	}
}

func TestParamStore_Read_notPointer(t *testing.T) {
	var config struct{}
	ps, err := NewParamStore()
	if err != nil {
		t.Fatal(err)
	}
	err = ps.Read(context.Background(), config)
	if err == nil {
		t.Error("Want error")
	}
}

func TestParamStore_Read_nilPointer(t *testing.T) {
	type config struct{}
	var cfg *config
	ps, err := NewParamStore()
	if err != nil {
		t.Fatal(err)
	}
	err = ps.Read(context.Background(), cfg)
	if err == nil {
		t.Error("Want error")
	}
}

func TestParamStore_Read_notStruct(t *testing.T) {
	var str string
	ps, err := NewParamStore()
	if err != nil {
		t.Fatal(err)
	}
	err = ps.Read(context.Background(), &str)
	if err == nil {
		t.Error("Want error")
	}
}

func TestParamStore_Read_ssmError(t *testing.T) {
	cfg := struct {
		Value string `ssm:"val"`
	}{}

	mock := &mockSSM{err: fmt.Errorf("error")}
	ps, _ := NewParamStore(
		WithClient(mock),
	)
	err := ps.Read(context.Background(), &cfg)
	if err == nil {
		t.Error("Want error")
	}
}

func stringParam(name, value string) ssm.Parameter {
	return ssm.Parameter{
		Name:  aws.String(name),
		Value: aws.String(value),
		Type:  ssm.ParameterTypeString,
	}
}

func stringListParam(name, value string) ssm.Parameter {
	return ssm.Parameter{
		Name:  aws.String(name),
		Value: aws.String(value),
		Type:  ssm.ParameterTypeStringList,
	}
}

func secureStringParam(name, value string) ssm.Parameter {
	return ssm.Parameter{
		Name:  aws.String(name),
		Value: aws.String(value),
		Type:  ssm.ParameterTypeSecureString,
	}
}

type value struct {
	path  string
	value interface{}
}

func check(t *testing.T, got interface{}, values []value) {
	t.Helper()
	root := reflect.ValueOf(got)
	for _, val := range values {
		field := root
		name := strings.Split(val.path, ".")
		for i, part := range name {
			if field.Kind() == reflect.Ptr {
				field = field.Elem()
			}
			field = field.FieldByName(part)
			if !field.IsValid() {
				// Field does not exist in struct
				t.Fatalf("%s: check value is not valid", strings.Join(name[:i+1], "."))
			}
		}
		got := field.Interface()
		if diff := cmp.Diff(got, val.value); diff != "" {
			t.Errorf("%s (-got +want)\n%s", val.path, diff)
		}
	}
}

type mockSSM struct {
	params []ssm.Parameter
	err    error
}

func (m *mockSSM) GetParametersRequest(input *ssm.GetParametersInput) ssm.GetParametersRequest {
	mockReq := &aws.Request{
		HTTPRequest:  &http.Request{},
		HTTPResponse: &http.Response{},
	}
	mockReq.Handlers.Send.PushBack(func(r *aws.Request) {
		if m.err != nil {
			r.Error = m.err
			return
		}
		var out []ssm.Parameter
		for _, name := range input.Names {
			for _, p := range m.params {
				if *p.Name != name {
					continue
				}
				if p.Type == ssm.ParameterTypeSecureString && !*input.WithDecryption {
					p.Value = aws.String("<ENCRYPTED>")
				}
				out = append(out, p)
			}
		}
		r.Data = &ssm.GetParametersOutput{
			Parameters: out,
		}
	})

	return ssm.GetParametersRequest{
		Request: mockReq,
	}
}
