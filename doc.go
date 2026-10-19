// Copyright (c) 2026 2Lines Sistem SRL. Licensed under the MIT License (see LICENSE).

// Package pluspanel is the Go client for the PlusPanel public API (/api/v1).
//
// The request and response types and the client methods are generated from the
// API's OpenAPI document (pluspanel.gen.go); NewWithAPIKey, Problem and
// CheckResponse are the small hand-written layer on top:
//
//	c, err := pluspanel.NewWithAPIKey("https://panel.example.com", os.Getenv("PLUSPANEL_TOKEN"))
//	if err != nil { ... }
//	res, err := c.ListServersWithResponse(ctx, &pluspanel.ListServersParams{})
//	if err != nil { ... }
//	if err := pluspanel.CheckResponse(res.HTTPResponse, res.Body); err != nil { ... }
//	for _, s := range *res.JSON200.Items { fmt.Println(s.Name) }
package pluspanel
