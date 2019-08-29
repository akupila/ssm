package ssm

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Client is the SSM client.
type Client interface {
	GetParametersRequest(input *ssm.GetParametersInput) ssm.GetParametersRequest
}

// A NotFoundError is returned when one or more of the requested parameters was
// not found.
type NotFoundError struct {
	names []string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("not found: %v", strings.Join(e.names, ", "))
}

// ParamStore reads configuration values from SSM Parameter Store.
type ParamStore struct {
	prefix string
	tag    string

	converters []func(param ssm.Parameter, value reflect.Value) (bool, error)

	cli Client
}

// An Option sets a configuration option in the ParamStore.
type Option func(s *ParamStore)

// NewParamStore creates a new parameter store.
//
// If WithTag was not passed, `ssm` is used as struct tag.
func NewParamStore(options ...Option) (*ParamStore, error) {
	s := &ParamStore{
		// Defaults
		tag: "ssm",
	}

	for _, opt := range options {
		opt(s)
	}

	// If cli was not set, load external config.
	if s.cli == nil {
		cfg, err := external.LoadDefaultAWSConfig()
		if err != nil {
			return nil, fmt.Errorf("load external aws config: %v", err)
		}
		WithClient(ssm.New(cfg))
	}

	return s, nil
}

// WithPrefix sets the prefix to use for all keys.
//
//   WithPrefix("dev")
//   WithPrefix("prod/app/db")
//   WithPrefix("test/auth/token")
//
// The prefix may contain a single / at the beginning or end.
func WithPrefix(prefix string) Option {
	return func(s *ParamStore) {
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		prefix = strings.TrimSuffix(prefix, "/")
		s.prefix = prefix
	}
}

// WithTag sets the struct tag to use for resolving schema.
func WithTag(tag string) Option {
	return func(s *ParamStore) {
		s.tag = tag
	}
}

// WithParseDuration parses a duration string to a time.Duration.
func WithParseDuration() Option {
	return func(s *ParamStore) {
		fn := func(param ssm.Parameter, value reflect.Value) (bool, error) {
			if value.Type() != reflect.TypeOf((time.Duration)(0)) {
				return false, nil
			}
			d, err := time.ParseDuration(*param.Value)
			if err != nil {
				return false, err
			}
			value.Set(reflect.ValueOf(d))
			return true, nil
		}
		s.converters = append(s.converters, fn)
	}
}

// WithParseTime parses a time string with the given layout to a time.Time.
func WithParseTime(layout string) Option {
	return func(s *ParamStore) {
		fn := func(param ssm.Parameter, value reflect.Value) (bool, error) {
			if value.Type() != reflect.TypeOf(time.Time{}) {
				return false, nil
			}
			t, err := time.Parse(layout, *param.Value)
			if err != nil {
				return false, err
			}
			value.Set(reflect.ValueOf(t))
			return true, nil
		}
		s.converters = append(s.converters, fn)
	}
}

// WithParseNumber enables parsing strings and lists of strings to ints and
// floats.
func WithParseNumber() Option {
	return func(s *ParamStore) {
		fn := func(param ssm.Parameter, value reflect.Value) (bool, error) {
			switch value.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				num, err := strconv.ParseInt(*param.Value, 10, 64)
				if err != nil {
					nerr := err.(*strconv.NumError)
					return false, fmt.Errorf("parse %q as int: %v", nerr.Num, nerr.Err)
				}
				value.SetInt(num)
				return true, nil
			case reflect.Float32, reflect.Float64:
				num, err := strconv.ParseFloat(*param.Value, 64)
				if err != nil {
					nerr := err.(*strconv.NumError)
					return false, fmt.Errorf("parse %q as float: %v", nerr.Num, nerr.Err)
				}
				value.SetFloat(num)
				return true, nil
			}
			return false, nil
		}
		s.converters = append(s.converters, fn)
	}
}

// WithClient sets the SSM client to use.
func WithClient(client Client) Option {
	return func(s *ParamStore) {
		s.cli = client
	}
}

// Read reads configuration values into the given target.
//
// The target must be a non-nil pointer to a struct.
func (s *ParamStore) Read(ctx context.Context, target interface{}) error {
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("target is not a pointer")
	}
	if val.IsNil() {
		return fmt.Errorf("target is a nil pointer")
	}
	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("target is not a pointer to a struct")
	}
	ty := val.Type()

	schema, err := s.schema(ty, s.prefix, nil)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(schema))
	for n := range schema {
		names = append(names, n)
	}

	input := &ssm.GetParametersInput{
		Names:          names,
		WithDecryption: aws.Bool(true),
	}
	resp, err := s.cli.GetParametersRequest(input).Send(ctx)
	if err != nil {
		return fmt.Errorf("read ssm: %v", err)
	}

	for _, param := range resp.Parameters {
		name := *param.Name
		index := schema[name]
		delete(schema, name)
		field := val
		for _, i := range index {
			field = field.Field(i)
			if field.Kind() == reflect.Ptr && field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
				field = field.Elem()
			}
		}
		if err := s.setValue(param, field); err != nil {
			return fmt.Errorf("%s: %v", *param.Name, err)
		}
	}
	if len(schema) > 0 {
		// Items were not deleted -> not found
		names = make([]string, 0, len(schema))
		for n := range schema {
			names = append(names, n)
		}
		return NotFoundError{names: names}
	}

	return nil
}

func (s *ParamStore) setValue(p ssm.Parameter, v reflect.Value) error {
	ty := v.Type()

	for _, conv := range s.converters {
		ok, err := conv(p, v)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}

	switch ty.Kind() {
	case reflect.String:
		switch p.Type {
		case ssm.ParameterTypeString, ssm.ParameterTypeSecureString:
			v.SetString(*p.Value)
		default:
			return fmt.Errorf("cannot assign %s to %s", p.Type, ty)
		}
	case reflect.Slice:
		if p.Type != ssm.ParameterTypeStringList {
			// Technically this would work, but we don't allow implicitly
			// converting the value.
			return fmt.Errorf("cannot set %s to %s", p.Type, v.Type())
		}
		parts := strings.Split(*p.Value, ",")
		n := len(parts)
		slice := reflect.MakeSlice(ty, n, n)
		for i, part := range parts {
			sliceParam := ssm.Parameter{
				Type:  ssm.ParameterTypeString,
				Value: aws.String(part),
			}
			if err := s.setValue(sliceParam, slice.Index(i)); err != nil {
				return fmt.Errorf("set slice index %d: %v", i, err)
			}
		}
		v.Set(slice)
	default:
		return fmt.Errorf("unsupported: %s", ty.Kind())
	}
	return nil
}

func (s *ParamStore) schema(t reflect.Type, keyPrefix string, index []int) (map[string][]int, error) {
	m := make(map[string][]int)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, ok := f.Tag.Lookup(s.tag)
		if !ok {
			continue
		}
		if f.PkgPath != "" {
			return nil, fmt.Errorf("field %q must be exported", f.Name)
		}
		name = keyPrefix + "/" + name
		ty := f.Type
		if ty.Kind() == reflect.Ptr {
			ty = ty.Elem()
		}

		if ty.Kind() == reflect.Struct && ty != reflect.TypeOf(time.Time{}) {
			// time.Time is also a struct - needs special case
			nested, err := s.schema(ty, name, append(index, i))
			if err != nil {
				return nil, err
			}
			for k, v := range nested {
				m[k] = v
			}
			continue
		}
		m[name] = append(index, i)

	}
	return m, nil
}
