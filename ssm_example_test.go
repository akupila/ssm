package ssm_test

import (
	"context"
	"log"
	"time"

	"github.com/akupila/ssm"
)

func Example() {
	type Config struct {
		Key string `ssm:"key"`
	}

	params, err := ssm.NewParamStore()
	if err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err := params.Read(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}

	// cfg.Key will now be the value of /key in ssm parameter store
}

func Example_options() {
	type Config struct {
		Key string `config:"key"`
	}

	params, err := ssm.NewParamStore(
		ssm.WithPrefix("dev"),
		ssm.WithTag("config"),
	)
	if err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err := params.Read(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}

	// cfg.Key will now be the value of /dev/key in ssm parameter store
}

func Example_nested() {
	type Config struct {
		DB struct {
			Host string `ssm:"host"`
			User string `ssm:"user"`
			Pass string `ssm:"password"`
			Port int    `ssm:"port"`
			Name string `ssm:"name"`
		} `ssm:"database"`
		Auth0 struct {
			ClientID     string `ssm:"client_id"`
			ClientSecret string `ssm:"client_secret"`
			Domain       string `ssm:"domain"`
		} `ssm:"auth0"`
	}

	params, err := ssm.NewParamStore(
		ssm.WithPrefix("dev"),
	)
	if err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err := params.Read(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}
}

func ExampleWithParseDuration() {
	type Config struct {
		Timeout time.Duration `ssm:"timeout"`
	}

	params, err := ssm.NewParamStore(
		ssm.WithParseDuration(),
	)
	if err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err := params.Read(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}
}

func ExampleWithParseTime() {
	type Config struct {
		Time time.Time `ssm:"timeout"`
	}

	params, err := ssm.NewParamStore(
		ssm.WithParseTime(time.RFC3339),
	)
	if err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err := params.Read(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}
}

func ExampleWithParseNumber() {
	type Config struct {
		Integer int     `ssm:"a"`
		Float   float64 `ssm:"b"`
	}

	params, err := ssm.NewParamStore(
		ssm.WithParseNumber(),
	)
	if err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err := params.Read(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}
}

func ExampleWithTag() {
	type Config struct {
		Username string `config:"username"`
		Password string `config:"password"`
	}

	params, err := ssm.NewParamStore(
		ssm.WithTag("config"),
	)
	if err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err := params.Read(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}
}
