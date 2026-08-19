package scripting

import (
	"fmt"

	"github.com/traefik/yaegi/interp"

	"github.com/thec1oud/billing/internal/shared/statemachine/scripting/env"
)

// ValidateGuardSource compiles source and confirms it defines script.Guard with
// the required signature, without executing it. Intended for use at
// definition-publish time so a broken script fails fast instead of at Fire().
func ValidateGuardSource(source string) error {
	return validateSource(source, "Guard")
}

// ValidateActionParamsSource compiles source and confirms it defines
// script.Params with the required signature, without executing it.
func ValidateActionParamsSource(source string) error {
	return validateSource(source, "Params")
}

func validateSource(source, funcName string) error {
	i := interp.New(interp.Options{})
	if err := i.Use(env.Exports); err != nil {
		return fmt.Errorf("load script env: %w", err)
	}
	if _, err := i.Eval(source); err != nil {
		return fmt.Errorf("compile script: %w", err)
	}

	fnVal, err := i.Eval("script." + funcName)
	if err != nil {
		return fmt.Errorf("resolve script.%s (did the script declare 'package script' and define %s?): %w", funcName, funcName, err)
	}

	switch funcName {
	case "Guard":
		if _, ok := fnVal.Interface().(func(map[string]any, map[string]any, map[string]any) bool); !ok {
			return fmt.Errorf("script.Guard must have signature func(context, payload, params map[string]any) bool")
		}
	case "Params":
		if _, ok := fnVal.Interface().(func(map[string]any, map[string]any, map[string]any) map[string]any); !ok {
			return fmt.Errorf("script.Params must have signature func(context, payload, params map[string]any) map[string]any")
		}
	}
	return nil
}
