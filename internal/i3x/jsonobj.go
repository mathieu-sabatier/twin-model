package i3x

import (
	"bytes"
	"encoding/json"
)

// jobj is an insertion-ordered JSON object. A Go map marshals its keys in sorted
// order, which would scramble declared member order (e.g. SerialNumber before
// State before CycleCount); jobj preserves the order keys were added, so output
// is a deterministic, byte-stable function of the source. It implements
// json.Marshaler by emitting compact JSON; json.MarshalIndent then re-indents the
// whole document uniformly, so nested jobj values are formatted consistently.
type jobj struct {
	keys []string
	vals []any
}

func newObj() *jobj { return &jobj{} }

// set appends a key/value pair and returns the object for chaining. Values may be
// any json-marshalable Go value, including nested *jobj.
func (o *jobj) set(key string, val any) *jobj {
	o.keys = append(o.keys, key)
	o.vals = append(o.vals, val)
	return o
}

// len reports the number of pairs.
func (o *jobj) len() int { return len(o.keys) }

// MarshalJSON emits the object as compact JSON in insertion order.
func (o *jobj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := marshalCompact(k)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		val, err := marshalCompact(o.vals[i])
		if err != nil {
			return nil, err
		}
		b.Write(val)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshalCompact marshals v to compact JSON without HTML-escaping (so `<`, `>`,
// and `&` stay literal — e.g. the "<ZoneNo>" placeholder pattern).
func marshalCompact(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil // Encode appends a newline
}
