# pluspanel-go

Go client for the [PlusPanel](https://pluspanel.net) public API (`/api/v1`).

The types and client methods are generated with
[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) from the API's OpenAPI document; a small
hand-written layer adds API-key authentication and typed errors. This repository is generated from the
PlusPanel platform: do not open pull requests against the generated code.

## Install

```sh
go get github.com/112tech/pluspanel-go
```

Requires Go 1.24 or later.

## Usage

Create an API key (`ppk_…`) in the panel, or sign in with the CLI (`pluspanel login`), then:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/112tech/pluspanel-go"
)

func main() {
	c, err := pluspanel.NewWithAPIKey("https://panel.example.com", os.Getenv("PLUSPANEL_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	params := &pluspanel.ListServersParams{}
	for {
		res, err := c.ListServersWithResponse(ctx, params)
		if err != nil {
			log.Fatal(err)
		}
		if err := pluspanel.CheckResponse(res.HTTPResponse, res.Body); err != nil {
			log.Fatal(err) // *pluspanel.Problem
		}
		for _, s := range *res.JSON200.Items {
			fmt.Println(s.Id, s.Name, s.Status)
		}
		if res.JSON200.NextCursor == nil {
			break
		}
		params.Cursor = res.JSON200.NextCursor
	}
}
```

- `NewWithAPIKey(baseURL, token, opts...)` sends `Authorization: Bearer <token>` and
  `User-Agent: pluspanel-go/<version>` on every request. A base URL without a path gets `/api/v1`
  appended. Plain `http://` is refused except for loopback hosts. Pass `pluspanel.WithHTTPClient` to set
  timeouts or a proxy, and `pluspanel.WithRequestEditorFn` to add headers (for example `Idempotency-Key`
  on a single call: pass the editor as the last argument of that call).
- Errors are [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details.
  `pluspanel.CheckResponse` turns any answer of 400 or above into a `*pluspanel.Problem`
  (`Type`, `Title`, `Status`, `Detail`, `Instance`, `Code`, `RequestID`); `pluspanel.IsProblem(err,
  "not_found")` tests the code. Quote `RequestID` when you contact support.
- Clients must ignore unknown response members and enum values: new ones are added without a new major
  version.

## License

MIT. See [LICENSE](LICENSE).
