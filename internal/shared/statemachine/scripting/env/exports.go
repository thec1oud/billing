package env

import (
	"reflect"

	"github.com/traefik/yaegi/interp"
)

// ImportPath is what a script writes to use this package: `import "statemachine/env"`,
// then calls e.g. `env.Contains(...)`.
const ImportPath = "statemachine/env"

// Exports is injected into every script interpreter via Interpreter.Use. This —
// not a deny-list of dangerous packages — is the sandbox: nothing beyond these
// symbols (plus Go language builtins like len/append/map literals, which need no
// import) is ever reachable from interpreted script text.
var Exports = interp.Exports{
	ImportPath + "/env": map[string]reflect.Value{
		"Contains":  reflect.ValueOf(Contains),
		"HasPrefix": reflect.ValueOf(HasPrefix),
		"HasSuffix": reflect.ValueOf(HasSuffix),
		"ToUpper":   reflect.ValueOf(ToUpper),
		"ToLower":   reflect.ValueOf(ToLower),
		"TrimSpace": reflect.ValueOf(TrimSpace),
	},
}
