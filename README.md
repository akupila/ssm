# ssm [![GoDoc](https://godoc.org/github.com/akupila/ssm?status.svg)](https://godoc.org/github.com/akupila/ssm)

[![Go Report Card](https://goreportcard.com/badge/github.com/akupila/ssm)](https://goreportcard.com/report/github.com/akupila/ssm)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](./LICENSE)

Read config values from [AWS System Manager Parameter Store][1] by names
defined using struct tags.

## Example

Parameters stored in SSM:

```
/dev
  /database
    /username
    /password
    /host
    /port
    /name
  /auth0
    /domain
    /client_id
    /client_secret
  /another
    /...
/prod
  ...
```

Read the required `dev` values:

```go
params := ssm.NewParamStore(
    ssm.WithPrefix("dev"),
    ssm.WithParseNumber(),
)

type Config struct {
    DB struct {
        User string `ssm:"username"`
        Pass string `ssm:"password"`
        Host string `ssm:"host"`
        Port int    `ssm:"port"`
        Name string `ssm:"name"`
    } `ssm:"database"`
    Auth0 struct {
        ClientID     string `ssm:"client_id"`
        ClientSecret string `ssm:"client_secret"`
    } `ssm:"auth0"`
    // Fields not included are not read from SSM
}

var cfg Config
if err := params.Read(context.Background(), &cfg); err != nil {
    // Handle error
}

// cfg is now ready to use
```

See [GoDoc][2] for more details.

[1]: https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html
[2]: http://godoc.org/github.com/akupila/ssm
