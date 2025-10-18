# Testing Guidelines

These guidelines apply to all Go code in this repository.

## Principles
- **Table-driven tests by default**: Write tests using table-driven style to cover multiple scenarios in a compact, readable block.
- **Deterministic and isolated**: Tests must not depend on external services by default. Prefer fakes, in-memory servers (e.g., pstest), or dependency injection.
- **Clear failure messages**: When comparing errors or values, include an extra line printing the actual values using `%#v` to aid debugging.
- **Fast CI**: Keep unit tests fast. Integration tests should be guarded (e.g., `-short`) or isolated behind build tags.

## Table-Driven Test Structure

Use this pattern for functions and methods:

```go
func TestSomething(t *testing.T) {
	type args struct {
		in1 string
		in2 int
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "valid", args: args{in1: "foo", in2: 1}, want: "ok", wantErr: false},
		{name: "invalid", args: args{in1: "", in2: -1}, want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DoSomething(tt.args.in1, tt.args.in2)
			if (err != nil) != tt.wantErr {
				t.Errorf("error mismatch\n gotErr=%#v\nwantErr=%#v\nerr=%#v", err != nil, tt.wantErr, err)
			}
			if got != tt.want {
				t.Errorf("result mismatch\n got=%#v\nwant=%#v", got, tt.want)
			}
		})
	}
}
```

For HTTP handlers:

```go
func TestHandler(t *testing.T) {
	tests := []struct {
		name string
		path string
		code int
		body string
	}{
		{name: "ok", path: "/healthz", code: 200, body: "ok"},
	}
	mux := http.NewServeMux()
	Register(mux)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.code {
				t.Errorf("status mismatch\n got=%#v\nwant=%#v", rec.Code, tt.code)
			}
			if rec.Body.String() != tt.body {
				t.Errorf("body mismatch\n got=%#v\nwant=%#v", rec.Body.String(), tt.body)
			}
		})
	}
}
```

## Pull Request Requirements
- **All new and changed functionality must include tests**.
- **Table-driven tests required** unless there is a strong reason otherwise (document why in the PR).
- **Error outputs** in assertions should include an extra line with `%#v` for actual values.
- **Avoid external dependencies** in unit tests. Use dependency injection or in-memory fakes.
- **Run `go test ./...` locally** before submitting. CI must pass.

## Coverage
- Aim to cover core code paths, error paths, and edge cases.
- For complex logic, prefer smaller focused tests over monolithic ones, still using table-driven style.
