package main

import (
	"bytes"
	"strings"
	"testing"
)

// approvedSpec is a small api/openapi.yaml in the shape of the real one: response and header $refs, a range
// status, a default response, media-type ranges, typed headers, a pattern and a format to assert, a non-local
// $ref and an operation without a body.
var approvedSpec = []string{
	"openapi: 3.1.0",
	"info: {title: t, version: 0.1.0}",
	"paths:",
	"  /healthz:",
	"    get:",
	"      operationId: getHealth",
	"      responses:",
	"        '200':",
	"          description: up",
	"          headers:",
	"            X-Request-Id: {$ref: '#/components/headers/XRequestId'}",
	"          content:",
	"            application/json:",
	"              schema: {$ref: '#/components/schemas/Health'}",
	"        4XX: {$ref: '#/components/responses/Problem'}",
	"        503: {$ref: '#/components/responses/Problem'}", // unquoted: YAML decodes the key as an int
	"  /things/{id}:",
	"    get:",
	"      operationId: getThing",
	"      responses:",
	"        '200':",
	"          description: thing",
	"          headers:",
	"            X-Rate-Limit: {schema: {$ref: '#/components/schemas/Count'}}",
	"            X-Cached: {schema: {type: boolean}}",
	"            X-Tags: {schema: {type: array, items: {type: integer}}}",
	"            X-Note: {description: no schema}",
	"          content:",
	"            application/json: {schema: {$ref: '#/components/schemas/Health'}}",
	"            text/*: {schema: {type: string, maxLength: 5}}",
	"            '*/*': {schema: {type: string, enum: [blob]}}",
	"        '410': {$ref: 'other.yaml#/components/responses/Gone'}",
	"        default: {$ref: '#/components/responses/Problem'}",
	"    delete:",
	"      operationId: deleteThing",
	"      responses:",
	"        '204': {description: gone}",
	"components:",
	"  schemas:",
	"    Health:",
	"      type: object",
	"      required: [status]",
	"      properties: {status: {type: string, enum: [ok]}}",
	"    Count: {type: integer, minimum: 0}",
	"    Problem:",
	"      type: object",
	"      required: [title, status]",
	"      properties:",
	"        title: {type: string}",
	"        status: {type: integer, minimum: 400, maximum: 599}",
	"        instance: {type: string, format: uri-reference}",
	"  headers:",
	"    XRequestId:",
	"      required: true",
	"      schema: {type: string, pattern: '^[A-Za-z0-9._-]+$'}",
	"  responses:",
	"    Problem:",
	"      description: problem",
	"      content:",
	"        application/problem+json:",
	"          schema: {$ref: '#/components/schemas/Problem'}",
}

const approvedDir = "apps/api/internal/platform/health/testdata/approved/"

// approvedRepo is a repo with approvedSpec and files: a bare name goes in approvedDir, a path is repo-relative.
func approvedRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	all := map[string][]string{"api/openapi.yaml": approvedSpec, layoutFile: templateLayout}
	for name, content := range files {
		if !strings.Contains(name, "/") {
			name = approvedDir + name
		}
		all[name] = []string{content}
	}
	return linkRepo(t, all)
}

const (
	okHeaders  = `"headers": {"Content-Type": "application/json", "X-Request-Id": "req-1"}`
	problemCT  = `"headers": {"Content-Type": "application/problem+json"}`
	okResponse = `{` + okHeaders + `, "body": {"status": "ok"}}`
)

func TestApprovedCheck(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []string // output lines; none means the check passes
	}{
		{"responses the contract allows", map[string]string{
			"getHealth.200.json":      okResponse,
			"getHealth.503.json":      `{` + problemCT + `, "body": {"title": "Service Unavailable", "status": 503, "instance": "urn:request-id:req-1"}}`,
			"getHealth.503.down.json": `{` + problemCT + `, "body": {"title": "Service Unavailable", "status": 503}}`,
			"getHealth.404.json":      `{"headers": {"Content-Type": "application/problem+json; charset=utf-8"}, "body": {"title": "Not Found", "status": 404}}`,
			"deleteThing.204.json":    `{"headers": {}}`,
		}, nil},
		{"default response, media-type ranges, typed headers", map[string]string{
			"getHealth.200.json":      okResponse,
			"getThing.500.json":       `{` + problemCT + `, "body": {"title": "Internal Server Error", "status": 500}}`,
			"getThing.200.json":       `{"headers": {"Content-Type": "application/json", "X-Rate-Limit": "12", "X-Cached": "true", "X-Tags": "1,2", "X-Note": "x"}, "body": {"status": "ok"}}`,
			"getThing.200.text.json":  `{"headers": {"Content-Type": "text/csv; charset=utf-8"}, "body": "a,b"}`,
			"getThing.200.other.json": `{"headers": {"Content-Type": "image/png"}, "body": "blob"}`,
			// the selftest's fixtures are not approved responses of this repo
			"scripts/harness/fixtures/x/files/" + approvedDir + "getGone.200.json": `{}`,
		}, nil},
		{"body breaks the schema", map[string]string{
			"getHealth.200.json": `{` + okHeaders + `, "body": {"status": "fine"}}`,
			"getHealth.503.json": `{` + problemCT + `, "body": {"status": 503, "instance": "%zz"}}`,
		}, []string{
			approvedDir + "getHealth.200.json:1: body/status: value must be 'ok' (api/openapi.yaml#/components/schemas/Health/properties/status) [api-approved-match-contract]",
			approvedDir + "getHealth.503.json:1: body/instance: '%zz' is not valid uri-reference: parse \"%zz\": invalid URL escape \"%zz\" (api/openapi.yaml#/components/schemas/Problem/properties/instance) [api-approved-match-contract]",
			approvedDir + "getHealth.503.json:1: body: missing property 'title' (api/openapi.yaml#/components/schemas/Problem) [api-approved-match-contract]",
		}},
		{"headers break the contract", map[string]string{
			"getHealth.200.json": `{"headers": {"Content-Type": "application/json"}, "body": {"status": "ok"}}`,
			"getHealth.201.json": `{"headers": {"Content-Type": "text/plain", "X-Request-Id": "bad id"}, "body": "ok"}`,
			"getThing.200.json":  `{"headers": {"Content-Type": "application/json", "X-Rate-Limit": "-1", "X-Cached": "yes"}, "body": {"status": "ok"}}`,
		}, []string{
			approvedDir + "getHealth.200.json:1: getHealth 200 requires the header X-Request-Id, the approved response has none [api-approved-match-contract]",
			approvedDir + "getHealth.201.json:1: getHealth declares no 201 response (nor 2XX or default) [api-approved-match-contract]",
			approvedDir + "getThing.200.json:1: headers/X-Cached: \"yes\" is not a JSON boolean (api/openapi.yaml#/paths/~1things~1{id}/get/responses/200/headers/X-Cached/schema) [api-approved-match-contract]",
			approvedDir + "getThing.200.json:1: headers/X-Rate-Limit: minimum: got -1, want 0 (api/openapi.yaml#/components/schemas/Count) [api-approved-match-contract]",
		}},
		{"content type and body presence", map[string]string{
			"getHealth.200.json":      `{"headers": {"Content-Type": "text/plain", "X-Request-Id": "bad id"}, "body": "ok"}`,
			"getHealth.503.json":      `{` + problemCT + `}`,
			"deleteThing.204.json":    `{"headers": {}, "body": {}}`,
			"getThing.200.text.json":  `{"headers": {"Content-Type": "text/csv"}, "body": "a,b,c,d"}`,
			"getThing.200.other.json": `{"headers": {"Content-Type": "image/png"}, "body": "png"}`,
		}, []string{
			approvedDir + "deleteThing.204.json:1: deleteThing 204 declares no body, the approved response has one [api-approved-match-contract]",
			approvedDir + "getHealth.200.json:1: Content-Type \"text/plain\" is not one of getHealth 200 application/json [api-approved-match-contract]",
			approvedDir + "getHealth.200.json:1: headers/X-Request-Id: 'bad id' does not match pattern '^[A-Za-z0-9._-]+$' (api/openapi.yaml#/components/headers/XRequestId/schema) [api-approved-match-contract]",
			approvedDir + "getHealth.503.json:1: getHealth 503 declares a body (application/problem+json), the approved response has no body or no Content-Type [api-approved-match-contract]",
			approvedDir + "getThing.200.other.json:1: body: value must be 'blob' (api/openapi.yaml#/paths/~1things~1{id}/get/responses/200/content/*~1*/schema) [api-approved-match-contract]",
			approvedDir + "getThing.200.text.json:1: body: maxLength: got 7, want 5 (api/openapi.yaml#/paths/~1things~1{id}/get/responses/200/content/text~1*/schema) [api-approved-match-contract]",
		}},
		{"non-local $ref", map[string]string{
			"getThing.410.json": `{` + problemCT + `, "body": {"title": "Gone", "status": 410}}`,
		}, []string{
			approvedDir + "getThing.410.json:1: /paths/~1things~1{id}/get/responses/410: $ref \"other.yaml#/components/responses/Gone\" is not local to api/openapi.yaml [api-approved-match-contract]",
		}},
		{"names and shape", map[string]string{
			"getHealth.json":          okResponse,
			"get-health.200.json":     okResponse, // not GetHealth: macOS file systems ignore case
			"getHealth.200.Down.json": okResponse,
			"getGone.200.json":        okResponse,
			"getHealth.200.json":      `{"head": {}}`,
		}, []string{
			approvedDir + "get-health.200.json:1: name is not <operationId>.<status>[.<case>].json [api-approved-match-contract]",
			approvedDir + "getGone.200.json:1: operationId getGone is not in api/openapi.yaml [api-approved-match-contract]",
			approvedDir + "getHealth.200.Down.json:1: name is not <operationId>.<status>[.<case>].json [api-approved-match-contract]",
			approvedDir + "getHealth.200.json:1: not an approved response ({\"headers\": {…}, \"body\": …}): json: unknown field \"head\" [api-approved-match-contract]",
			approvedDir + "getHealth.json:1: name is not <operationId>.<status>[.<case>].json [api-approved-match-contract]",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			bad, err := approvedCheck(approvedRepo(t, tt.files), &out)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Split(strings.TrimSpace(out.String()), "\n")
			if out.Len() == 0 {
				got = nil
			}
			if bad != (len(tt.want) > 0) || strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("bad=%v, output:\n%s\nwant:\n%s", bad, out.String(), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestApprovedCheckExitCodes(t *testing.T) {
	ok := approvedRepo(t, map[string]string{"getHealth.200.json": okResponse})
	bad := approvedRepo(t, map[string]string{"getHealth.200.json": `{}`})
	none := approvedRepo(t, nil) // the template ships getHealth's: none means layout or pathspec drift
	onlyFixtures := approvedRepo(t, map[string]string{"scripts/harness/fixtures/x/files/" + approvedDir + "getHealth.200.json": okResponse})
	noSpec := linkRepo(t, map[string][]string{layoutFile: templateLayout})
	badSpec := linkRepo(t, map[string][]string{"api/openapi.yaml": {"paths: [unclosed"}, layoutFile: templateLayout})
	// The spec is where the layout's SPEC says, not at a fixed path.
	movedSpec := linkRepo(t, map[string][]string{"contract/v1.yaml": approvedSpec, layoutFile: withLayout("SPEC", "contract/v1.yaml"),
		approvedDir + "getHealth.200.json": {okResponse}})
	noLayout := linkRepo(t, map[string][]string{"api/openapi.yaml": approvedSpec, approvedDir + "getHealth.200.json": {okResponse}})
	for _, tt := range []struct {
		args       []string
		want       int
		wantStderr string
	}{
		{[]string{"approved-check", ok}, 0, ""},
		{[]string{"approved-check", bad}, 1, ""},
		{[]string{"approved-check", none}, 2, "no approved response matches */testdata/approved/* (fixtures skipped)"},
		{[]string{"approved-check", onlyFixtures}, 2, "restore them with `make approve`"},
		{[]string{"approved-check", noSpec}, 2, "api/openapi.yaml: "},
		{[]string{"approved-check", movedSpec}, 0, ""},
		{[]string{"approved-check", noLayout}, 2, layoutFile},
		{[]string{"approved-check", badSpec}, 2, ""},
		{[]string{"approved-check", t.TempDir()}, 2, ""},
		{[]string{"approved-check"}, exitUsage, ""},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(""), &stdout, &stderr); code != tt.want || !strings.Contains(stderr.String(), tt.wantStderr) {
			t.Errorf("%v: exit %d, want %d (%s%s)", tt.args, code, tt.want, stdout.String(), stderr.String())
		}
	}
}
