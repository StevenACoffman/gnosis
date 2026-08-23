package standards

import (
	"reflect"
	"sort"
	"strings"

	"github.com/StevenACoffman/skillet/errs"
)

// checkRationales reports every Value in cfg whose rationale is missing or blank.
//
// Requires: cfg is a struct or pointer to one.
// Ensures: returns EINVALID naming every offending key at once, sorted, because a
// loader that reports one missing rationale per run turns a five-value file into
// five edit-and-rerun cycles. Reports nothing for a struct holding no Values.
//
// This walks by reflection rather than by an explicit list of fields. A list is a
// second place to remember, and the failure it permits — a threshold added to the
// file and to the struct but not to the list — produces exactly the unjustified
// value this check exists to prevent.
func checkRationales(op string, cfg any) error {
	var missing []string
	walk(reflect.ValueOf(cfg), "", &missing)
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return &errs.Error{
		Code:    errs.EINVALID,
		Message: op + ": value(s) with no rationale: " + strings.Join(missing, ", "),
	}
}

// walk descends v, appending the dotted path of every Value lacking a rationale.
func walk(v reflect.Value, path string, missing *[]string) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	// A Value is itself a struct, so the interface check comes before descending —
	// otherwise the walk would recurse into it and find nothing to report.
	if j, ok := v.Interface().(justified); ok {
		if strings.TrimSpace(j.justification()) == "" {
			*missing = append(*missing, path)
		}
		return
	}
	t := v.Type()
	for i := range v.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		walk(v.Field(i), join(path, key(&f)), missing)
	}
}

// key is the field's name as it appears in the file, so a reported path is one a
// reader can search for.
func key(f *reflect.StructField) string {
	tag, _, _ := strings.Cut(f.Tag.Get("toml"), ",")
	if tag != "" && tag != "-" {
		return tag
	}
	return strings.ToLower(f.Name)
}

func join(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
