package main

// approved-check (api-approved-match-contract): every approved response file is a response api/openapi.yaml
// allows. The files are `<pkg>/testdata/approved/<operationId>.<status>[.<case>].json`, written by
// `make approve` from the handler tests; apps/api/internal/platform/approvaltest (File) writes this format
// (rules.md#api-responses-approved, ADR 0022):
//
//	{"headers": {"<Name>": "<value>", …}, "body": <JSON value, absent for an empty body>}
//
// For each file: the operationId exists; the operation declares the status (exactly, else `<n>XX`, else
// `default`); the Content-Type is one of that response's media types (exactly, else `<type>/*`, else `*/*`)
// and the body is valid against its schema (JSON Schema 2020-12, the OpenAPI 3.1 dialect, formats asserted);
// every required header is present and every declared header's value is valid against its schema, parsed
// first when the schema's type is integer, number or boolean.
//
// Not checked yet: header values whose schema is an array or object (OpenAPI's `simple` style is not parsed);
// headers the response does not declare (TD-16); operations under a path item that is itself a `$ref` (not
// followed). Local `$ref`s (to components) are followed. Nothing is fetched: the spec is the only schema
// resource. Approved files under scripts/harness/fixtures are skipped. Run from `make approved-check` (make
// check). Fails closed (exit 2) when the spec cannot be read or parsed, or when there is no approved file at
// all: the template ships getHealth's, so none means the layout or the pathspec drifted.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	specFile         = "api/openapi.yaml"
	approvedPathspec = "*/testdata/approved/*"
	// specURL names the spec as the one schema resource; local refs resolve against it.
	specURL = "file:///harness/api/openapi.yaml"
)

var (
	// <operationId>.<status>[.<case>].json; the case charset is approvaltest's caseRE.
	approvedNameRE = regexp.MustCompile(`^([a-z][A-Za-z0-9]*)\.([1-5][0-9][0-9])(\.[a-z][a-z0-9-]*)?\.json$`)
	httpMethods    = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
)

type approvedChecker struct {
	out     io.Writer
	bad     bool
	doc     any
	ops     map[string]string // operationId → JSON pointer of the operation
	c       *jsonschema.Compiler
	schemas map[string]*jsonschema.Schema // JSON pointer → compiled schema
}

func (a *approvedChecker) report(file, format string, args ...any) {
	fmt.Fprintf(a.out, "%s:1: %s [api-approved-match-contract]\n", file, fmt.Sprintf(format, args...))
	a.bad = true
}

// approvedCheck prints one line per finding to stdout and reports whether there was any. root is the repo.
func approvedCheck(root string, stdout io.Writer) (bool, error) {
	doc, err := loadSpec(filepath.Join(root, specFile))
	if err != nil {
		return false, fmt.Errorf("%s: %w", specFile, err)
	}
	a := &approvedChecker{out: stdout, doc: doc, ops: map[string]string{}, schemas: map[string]*jsonschema.Schema{}}
	paths, _ := pointer(doc, "/paths").(map[string]any)
	for p, item := range paths {
		for _, m := range httpMethods {
			if id, ok := pointer(item, "/"+m+"/operationId").(string); ok {
				a.ops[id] = "/paths/" + escape(p) + "/" + m
			}
		}
	}
	a.c = jsonschema.NewCompiler()
	a.c.DefaultDraft(jsonschema.Draft2020)
	a.c.AssertFormat()
	a.c.UseLoader(jsonschema.SchemeURLLoader{}) // no file or network loads: every $ref stays in the spec
	if err := a.c.AddResource(specURL, doc); err != nil {
		return false, fmt.Errorf("%s: %w", specFile, err)
	}

	files, err := gitFiles(root, approvedPathspec)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		return false, fmt.Errorf("no approved response matches %s (fixtures skipped), though the template ships apps/api/internal/platform/health/testdata/approved/getHealth.*.json: restore them with `make approve`, or, if the layout moved, update approvedPathspec in approvedcheck.go (Harness change: Y)", approvedPathspec)
	}
	sort.Strings(files)
	for _, f := range files {
		if err := a.check(root, f); err != nil {
			return false, err
		}
	}
	return a.bad, nil
}

// loadSpec parses the YAML spec into the JSON data model the validator expects (string keys, json.Number).
func loadSpec(file string) (any, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var v any
	if err := yaml.Unmarshal(src, &v); err != nil {
		return nil, err
	}
	v, err = jsonKeys(v)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if _, ok := doc.(map[string]any); !ok {
		return nil, errors.New("not a YAML mapping")
	}
	return doc, nil
}

// jsonKeys turns YAML mappings into map[string]any: an unquoted `200:` key decodes as an int.
func jsonKeys(v any) (any, error) {
	switch v := v.(type) {
	case map[string]any:
		for k, e := range v {
			e, err := jsonKeys(e)
			if err != nil {
				return nil, err
			}
			v[k] = e
		}
		return v, nil
	case map[any]any:
		m := make(map[string]any, len(v))
		for k, e := range v {
			e, err := jsonKeys(e)
			if err != nil {
				return nil, err
			}
			m[fmt.Sprint(k)] = e
		}
		return m, nil
	case []any:
		for i, e := range v {
			e, err := jsonKeys(e)
			if err != nil {
				return nil, err
			}
			v[i] = e
		}
	}
	return v, nil
}

func (a *approvedChecker) check(root, file string) error {
	m := approvedNameRE.FindStringSubmatch(path.Base(file))
	if m == nil {
		a.report(file, "name is not <operationId>.<status>[.<case>].json")
		return nil
	}
	opID, status := m[1], m[2]
	op, ok := a.ops[opID]
	if !ok {
		a.report(file, "operationId %s is not in %s", opID, specFile)
		return nil
	}
	resp, key := "", ""
	for _, k := range []string{status, status[:1] + "XX", "default"} {
		if pointer(a.doc, op+"/responses/"+k) != nil {
			resp, key = op+"/responses/"+k, k
			break
		}
	}
	if resp == "" {
		a.report(file, "%s declares no %s response (nor %sXX or default)", opID, status, status[:1])
		return nil
	}
	if resp, ok = a.resolve(file, resp); !ok {
		return nil
	}

	src, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		return err
	}
	var got struct {
		Headers map[string]string `json:"headers"`
		Body    json.RawMessage   `json:"body"`
	}
	d := json.NewDecoder(bytes.NewReader(src))
	d.DisallowUnknownFields()
	if err := d.Decode(&got); err != nil {
		a.report(file, "not an approved response ({\"headers\": {…}, \"body\": …}): %v", err)
		return nil
	}
	headers := map[string]string{}
	for k, v := range got.Headers {
		headers[strings.ToLower(k)] = v
	}
	where := fmt.Sprintf("%s %s", opID, key)

	content, _ := pointer(a.doc, resp+"/content").(map[string]any)
	ct, hasCT := headers["content-type"]
	switch {
	case len(content) == 0 && got.Body != nil:
		a.report(file, "%s declares no body, the approved response has one", where)
	case len(content) > 0 && (got.Body == nil || !hasCT):
		a.report(file, "%s declares a body (%s), the approved response has no body or no Content-Type", where, strings.Join(keys(content), ", "))
	case len(content) > 0:
		media, ok := mediaType(content, ct)
		if !ok {
			a.report(file, "Content-Type %q is not one of %s %s", ct, where, strings.Join(keys(content), ", "))
			break
		}
		if err := a.validate(file, resp+"/content/"+escape(media)+"/schema", "body", got.Body); err != nil {
			return err
		}
	}

	declared, _ := pointer(a.doc, resp+"/headers").(map[string]any)
	for _, name := range keys(declared) {
		if strings.EqualFold(name, "Content-Type") { // OpenAPI ignores a Content-Type header definition
			continue
		}
		h, ok := a.resolve(file, resp+"/headers/"+escape(name))
		if !ok {
			continue
		}
		v, present := headers[strings.ToLower(name)]
		if !present {
			if pointer(a.doc, h+"/required") == true {
				a.report(file, "%s requires the header %s, the approved response has none", where, name)
			}
			continue
		}
		if pointer(a.doc, h+"/schema") == nil {
			continue
		}
		sch, ok := a.resolve(file, h+"/schema")
		if !ok {
			continue
		}
		var raw []byte
		switch typ, _ := pointer(a.doc, sch+"/type").(string); typ {
		case "integer", "number", "boolean": // a header value is text; the schema types the value it spells
			if _, err := jsonschema.UnmarshalJSON(strings.NewReader(v)); err != nil {
				a.report(file, "headers/%s: %q is not a JSON %s (%s#%s)", name, v, typ, specFile, sch)
				continue
			}
			raw = []byte(v)
		case "array", "object": // OpenAPI's `simple` style is not parsed yet (file comment)
			continue
		default:
			raw, _ = json.Marshal(v)
		}
		if err := a.validate(file, sch, "headers/"+name, raw); err != nil {
			return err
		}
	}
	return nil
}

// mediaType is the key of content that the Content-Type ct selects: exactly, else `<type>/*`, else `*/*`.
func mediaType(content map[string]any, ct string) (string, bool) {
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return "", false
	}
	typ, _, _ := strings.Cut(mt, "/")
	for _, k := range []string{mt, typ + "/*", "*/*"} {
		if _, ok := content[k]; ok {
			return k, true
		}
	}
	return "", false
}

// resolve follows local $refs from the object at ptr to the object they name.
func (a *approvedChecker) resolve(file, ptr string) (string, bool) {
	for range 10 {
		ref, ok := pointer(a.doc, ptr+"/$ref").(string)
		if !ok {
			return ptr, true
		}
		if !strings.HasPrefix(ref, "#/") {
			a.report(file, "%s: $ref %q is not local to %s", ptr, ref, specFile)
			return "", false
		}
		ptr = ref[1:]
		if pointer(a.doc, ptr) == nil {
			a.report(file, "$ref %q names nothing in %s", ref, specFile)
			return "", false
		}
	}
	a.report(file, "%s: $ref chain longer than 10", ptr)
	return "", false
}

// validate reports every way value breaks the schema at ptr (none when there is no schema).
func (a *approvedChecker) validate(file, ptr, what string, value []byte) error {
	if pointer(a.doc, ptr) == nil {
		return nil
	}
	sch, ok := a.schemas[ptr]
	if !ok {
		var err error
		if sch, err = a.c.Compile(specURL + "#" + ptr); err != nil {
			return fmt.Errorf("%s#%s: %w", specFile, ptr, err)
		}
		a.schemas[ptr] = sch
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(value))
	if err != nil {
		a.report(file, "%s: %v", what, err)
		return nil
	}
	var ve *jsonschema.ValidationError
	if err := sch.Validate(v); errors.As(err, &ve) {
		var found []string
		for _, leaf := range leaves(ve) {
			loc := ""
			for _, tok := range leaf.InstanceLocation {
				loc += "/" + tok
			}
			// The schema that rejected the value: the leaf's own location, refs followed, as a plain JSON
			// pointer (the validator percent-encodes `{`, `}` and `*`).
			at := strings.TrimPrefix(leaf.SchemaURL, specURL)
			if u, err := url.PathUnescape(at); err == nil {
				at = u
			}
			found = append(found, fmt.Sprintf("%s%s: %s (%s%s)", what, loc, leaf.ErrorKind.LocalizedString(printer), specFile, at))
		}
		sort.Strings(found) // the validator's order follows map iteration
		for _, f := range found {
			a.report(file, "%s", f)
		}
	} else if err != nil {
		return err
	}
	return nil
}

// printer renders validation messages; the library's own default is English too.
var printer = message.NewPrinter(language.English)

// leaves are the errors that say what is wrong; the rest only group them. (BasicOutput drops the cause of a
// `$ref` with a single cause, so walk the tree.)
func leaves(e *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(e.Causes) == 0 {
		return []*jsonschema.ValidationError{e}
	}
	var out []*jsonschema.ValidationError
	for _, c := range e.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}

// pointer returns the value at the JSON pointer ptr ("" is the whole document), or nil.
func pointer(v any, ptr string) any {
	if ptr == "" {
		return v
	}
	for _, tok := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		if v, ok = m[strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")]; !ok {
			return nil
		}
	}
	return v
}

// escape is a JSON pointer token for key.
func escape(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
