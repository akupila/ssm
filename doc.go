// Package ssm provides a way to read config values from AWS Systems Manager
// Parameter Store.
//
// Struct tags
//
// Struct tags determine what parameters to get from SSM:
//
//   type Config struct {
//       Host     string `ssm:"host"`
//       User     string `ssm:"user"`
//       Password string `ssm:"password"`
//       Port     int    `ssm:"port"`
//       Name     string `ssm:"name"`
//       Scheme   string `ssm:"scheme"`
//   }
//
// The name of the struct tag to use can be set by passing WithTag to
// NewParamStore. Defaults to `ssm`.
//
// Nested values
//
// Nested struct value are allowed. When present, the name to read from SSM is
// constructed by the path to the value:
//
//   type Config struct {
//       DynamoDB struct {
//           Table string `ssm:"table"`                  // /dynamo_db/table
//       } `ssm:"dynamo_db"`
//       Auth0 struct {
//           ClientID     string   `ssm:"client_id"`     // /auth0/client_id
//           ClientSecret string   `ssm:"client_secret"` // /auth0/client_secret
//           Domain       string   `ssm:"domain"`        // /auth0/domain
//           Scope        []string `ssm:"scope"`         // /auth0/scope
//       } `ssm:"auth0"`
//   }
//
// Options
//
// The behavior can be modified by passing options to NewParamStore. If no
// options are passed, the external aws config is read for the SSM client, and
// ssm is used as the struct tag.
//
// WithPrefix allows all keys to be prefixed with a value. Given the following
// structure in SSM:
//
//   /
//     /dev
//       /myapp
//         /db
//           /user
//           /pass
//
// The values can be read for myapp using WithPrefix("dev/myapp"):
//
//   type Config struct {
//       DB struct {
//           User string `ssm:"user"`
//           Pass string `ssm:"user"`
//       } `ssm:"db"`
//   }
//
// Times and durations can be parsed using WithParseTime and WithParseDuration.
//
// Slices
//
// If the parameter type is StringList, the value can be assigned to a slice.
// Conversion rules apply to items within the slice, allowing for example []int
// to be used.
//
// https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html
package ssm
