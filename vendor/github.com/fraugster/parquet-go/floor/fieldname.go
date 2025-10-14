package floor

import (
	"reflect"
	"strings"

	"github.com/fraugster/parquet-go/parquetschema/autoschema"
)

var fieldNameFunc = fieldNameToLower

func fieldNameToLower(field reflect.StructField) (name string) {
	defer func() {
		if name == "" {
			name = strings.ToLower(field.Name)
		}
	}()

	tagFieldMap, err := autoschema.CreateTagFieldMap(field, "")
	if err == nil {
		name = tagFieldMap[autoschema.StructTagNameKey]
	}

	return
}
