package ro

import (
	"encoding"
	"fmt"
	"net/url"
	"reflect"
	"strconv"

	"cameo/internal/protocol"
)

// decodeQuery fills the `query:"name"` tagged fields of the struct pointed to by target, including fields of embedded structs.
// Supported field types: string, bool, signed integers, encoding.TextUnmarshaler (uuid.UUID, enums) and pointers to them.
func decodeQuery(values url.Values, target any) error {
	value := reflect.ValueOf(target).Elem()
	if value.Kind() != reflect.Struct {
		return nil
	}
	return decodeQueryStruct(values, value)
}

func decodeQueryStruct(values url.Values, value reflect.Value) error {
	for i := range value.NumField() {
		field := value.Type().Field(i)
		name := field.Tag.Get("query")
		if name == "" && field.Anonymous && field.Type.Kind() == reflect.Struct {
			if err := decodeQueryStruct(values, value.Field(i)); err != nil {
				return err
			}
			continue
		}
		if name == "" || !values.Has(name) {
			continue
		}

		if err := setQueryValue(value.Field(i), values.Get(name)); err != nil {
			return protocol.ErrorResponse{
				Code:    protocol.InvalidRequest,
				Message: fmt.Sprintf("invalid query parameter %q: %v", name, err),
			}
		}
	}

	return nil
}

func setQueryValue(field reflect.Value, raw string) error {
	if field.Kind() == reflect.Pointer {
		element := reflect.New(field.Type().Elem())
		if err := setQueryValue(element.Elem(), raw); err != nil {
			return err
		}
		field.Set(element)
		return nil
	}

	if unmarshaler, ok := reflect.TypeAssert[encoding.TextUnmarshaler](field.Addr()); ok {
		return unmarshaler.UnmarshalText([]byte(raw))
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(parsed)
	default:
		return fmt.Errorf("unsupported query field kind %s", field.Kind())
	}

	return nil
}
