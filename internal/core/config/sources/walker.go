package sources

import (
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	yaml "gopkg.in/yaml.v2"
)

func isDynamicValue(v interface{}) bool {
	m, ok := v.(map[interface{}]interface{})
	if !ok {
		return false
	}

	return m["#from"] != nil
}

func renderDynamicValues(config []byte, resolve resolveFunc) ([]byte, error) {
	var content map[interface{}]interface{}
	err := yaml.Unmarshal(config, &content)
	if err != nil {
		return nil, err
	}

	w := walker{
		resolve:   resolve,
		pathsSeen: make(map[string]bool),
	}
	resolvedContent, err := w.injectDynamicValues(content)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(resolvedContent)
}

// Use a struct to maintain context about the tree walking so that we don't
// have to pass around so many variables.
type walker struct {
	pathsSeen map[string]bool
	resolve   resolveFunc
}

func (w *walker) doResolution(rawSpec RawDynamicValueSpec) ([]interface{}, *dynamicValueSpec, error) {
	log.Debugf("Resolving %s", rawSpec)
	values, path, spec, err := w.resolve(rawSpec)
	if err != nil {
		return nil, spec, err
	}

	if w.pathsSeen[path] {
		var paths []string
		for k := range w.pathsSeen {
			paths = append(paths, k)
		}
		return nil, spec, fmt.Errorf("Dynamic value paths %s have a circular dependency", strings.Join(paths, "; "))
	}

	// Set the current path in our path set before we go recursing into the
	// resolved value and pop it out once we're done resolving this line of
	// dynamic values.
	w.pathsSeen[path] = true

	log.Debugf("Resolved %s to %s", rawSpec, spew.Sdump(values))
	var out []interface{}
	for i := range values {
		val, err := w.injectDynamicValues(values[i])
		if err != nil {
			return nil, spec, err
		}
		out = append(out, val)
	}
	delete(w.pathsSeen, path)

	log.Debugf("Final resolution of %s is %s", rawSpec, spew.Sdump(out))
	return out, spec, nil
}

func (w *walker) injectDynamicValues(v interface{}) (interface{}, error) {
	if s, ok := v.([]interface{}); ok {
		return w.injectDynamicValuesInSlice(s)
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		return w.injectDynamicValuesInMap(m)
	}
	return v, nil
}

func (w *walker) injectDynamicValuesInMap(m map[interface{}]interface{}) (map[interface{}]interface{}, error) {
	out := make(map[interface{}]interface{})
	for k, v := range m {
		if isDynamicValue(v) {
			values, spec, err := w.doResolution(RawDynamicValueSpec(v))
			if err != nil {
				return nil, errors.WithMessage(err, fmt.Sprintf("could not process key '%s'", k))
			}
			if spec.Flatten {
				if !strings.HasPrefix(k.(string), "_") {
					return nil, fmt.Errorf(
						"When flattening a map into another map, the key should "+
							"start with '_' to make the intention clear, you used '%s'", k)
				}
				for i := range values {
					if m, ok := values[i].(map[interface{}]interface{}); ok {
						for k2, v2 := range m {
							out[k2] = v2
						}
					} else {
						return nil, fmt.Errorf("Cannot flatten non-map at key '%s' in map context", k)
					}
				}
			} else {
				merged, err := mergeValues(values)
				if err != nil {
					return nil, err
				}

				val, err := w.injectDynamicValues(merged)
				if err != nil {
					return nil, err
				}
				out[k] = val
			}
		} else {
			val, err := w.injectDynamicValues(v)
			if err != nil {
				return nil, err
			}

			out[k] = val
		}
	}

	return out, nil
}

func (w *walker) injectDynamicValuesInSlice(v []interface{}) ([]interface{}, error) {
	out := make([]interface{}, 0, len(v))

	for i := range v {
		if isDynamicValue(v[i]) {
			values, spec, err := w.doResolution(RawDynamicValueSpec(v[i]))
			if err != nil {
				return nil, err
			}
			if spec.Flatten {
				for j := range values {
					slice, ok := values[j].([]interface{})
					if !ok {
						slice = []interface{}{values[j]}
					}
					out = append(out[:i], append(slice, out[i:]...)...)
				}
			} else {
				out = append(out, values...)
			}
		} else {
			val, err := w.injectDynamicValues(v[i])
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		}
	}
	return out, nil
}

func mergeValues(v []interface{}) (interface{}, error) {
	if len(v) == 0 {
		return nil, nil
	}

	if _, ok := v[0].([]interface{}); ok {
		return mergeSlices(v)
	} else if _, ok := v[0].(map[interface{}]interface{}); ok {
		return mergeMaps(v)
	} else {
		return v[0], nil
	}
}

func mergeSlices(v []interface{}) ([]interface{}, error) {
	var out []interface{}
	for i := range v {
		if s, ok := v[i].([]interface{}); ok {
			out = append(out, s...)
		} else {
			return nil, errors.New("Cannot merge a collection and slice")
		}
	}
	return out, nil
}

func mergeMaps(v []interface{}) (map[interface{}]interface{}, error) {
	out := make(map[interface{}]interface{})
	for i := range v {
		if m, ok := v[i].(map[interface{}]interface{}); ok {
			for k, v := range m {
				out[k] = v
			}
		} else {
			return nil, errors.New("Cannot merge a collection and slice")
		}
	}
	return out, nil
}
