package autoschema

import (
	"reflect"
	"time"

	goparquet "github.com/fraugster/parquet-go"
	"github.com/fraugster/parquet-go/parquet"
	"github.com/fraugster/parquet-go/parquetschema"
	"github.com/pkg/errors"
)

// GenerateSchema auto-generates a schema definition for a provided object's type
// using reflection. The generated schema is meant to be compatible with
// github.com/fraugster/parquet-go/floor's reflection-based marshalling/unmarshalling.
func GenerateSchema(obj interface{}) (*parquetschema.SchemaDefinition, error) {
	valueObj := reflect.ValueOf(obj)
	columns, err := generateSchema(valueObj.Type())
	if err != nil {
		return nil, errors.Wrap(err, "generating schema")
	}

	schemaDef := &parquetschema.SchemaDefinition{
		RootColumn: &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Name: "autogen_schema",
			},
			Children: columns,
		},
	}
	if err = schemaDef.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid schema")
	}

	return schemaDef, nil
}

func generateSchema(objType reflect.Type) ([]*parquetschema.ColumnDefinition, error) {
	if objType.Kind() == reflect.Ptr {
		objType = objType.Elem()
	}

	if objType.Kind() != reflect.Struct {
		return nil, errors.New("can't generate schema: provided object needs to be of type struct or *struct")
	}

	columns := []*parquetschema.ColumnDefinition{}

	for i := 0; i < objType.NumField(); i++ {
		field := objType.Field(i)

		column, err := generateField(field, field.Type, "")
		if err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}

	return columns, nil
}

func generateField(field reflect.StructField, fieldType reflect.Type, fieldName string) (column *parquetschema.ColumnDefinition, err error) {
	defer func() {
		if err == nil {
			if err = errors.Wrapf(parseParquetTag(field, fieldType, column), "parsing Parquet struct tag for field '%s'", field.Name); err != nil {
				column = nil
			}
		}
	}()

	switch fieldType.Kind() {
	case reflect.Bool:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_BOOLEAN),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
			},
		}
	case reflect.Int:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT64),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_INT_64),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 64,
						IsSigned: true,
					},
				},
			},
		}
	case reflect.Int8:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT32),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_INT_16),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 8,
						IsSigned: true,
					},
				},
			},
		}
	case reflect.Int16:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT32),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_INT_16),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 16,
						IsSigned: true,
					},
				},
			},
		}
	case reflect.Int32:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT32),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_INT_32),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 32,
						IsSigned: true,
					},
				},
			},
		}
	case reflect.Int64:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT64),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_INT_64),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 64,
						IsSigned: true,
					},
				},
			},
		}
	case reflect.Uint:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT32),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_32),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 32,
						IsSigned: false,
					},
				},
			},
		}
	case reflect.Uint8:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT32),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_16),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 8,
						IsSigned: false,
					},
				},
			},
		}
	case reflect.Uint16:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT32),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_16),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 16,
						IsSigned: false,
					},
				},
			},
		}
	case reflect.Uint32:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT32),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_32),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 32,
						IsSigned: false,
					},
				},
			},
		}
	case reflect.Uint64:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_INT64),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_64),
				LogicalType: &parquet.LogicalType{
					INTEGER: &parquet.IntType{
						BitWidth: 64,
						IsSigned: false,
					},
				},
			},
		}
	case reflect.Uintptr:
		err = errors.New("unsupported type uintptr")
	case reflect.Float32:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_FLOAT),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
			},
		}
	case reflect.Float64:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_DOUBLE),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
			},
		}
	case reflect.Complex64:
		err = errors.New("unsupported type complex64")
	case reflect.Complex128:
		err = errors.New("unsupported type complex128")
	case reflect.Chan:
		err = errors.New("unsupported type chan")
	case reflect.Func:
		err = errors.New("unsupported type func")
	case reflect.Interface:
		err = errors.New("unsupported type interface")
	case reflect.Map:
		var keyType *parquetschema.ColumnDefinition
		keyType, err = generateField(field, fieldType.Key(), "key")
		if err != nil {
			return
		}

		var valueType *parquetschema.ColumnDefinition
		valueType, err = generateField(field, fieldType.Elem(), "value")
		if err != nil {
			return
		}

		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_OPTIONAL),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_MAP),
				LogicalType: &parquet.LogicalType{
					MAP: &parquet.MapType{},
				},
			},
			Children: []*parquetschema.ColumnDefinition{
				{
					SchemaElement: &parquet.SchemaElement{
						Name:           "key_value",
						RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REPEATED),
						ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_MAP_KEY_VALUE),
					},
					Children: []*parquetschema.ColumnDefinition{
						keyType,
						valueType,
					},
				},
			},
		}
	case reflect.Ptr:
		column, err = generateField(field, fieldType.Elem(), fieldName)
		if err != nil {
			return
		}
		column.SchemaElement.RepetitionType = parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_OPTIONAL)
	case reflect.Slice, reflect.Array:
		if fieldType.Elem().Kind() == reflect.Uint8 {
			switch fieldType.Kind() {
			case reflect.Slice:
				// Handle special case for []byte.
				column = &parquetschema.ColumnDefinition{
					SchemaElement: &parquet.SchemaElement{
						Type:           parquet.TypePtr(parquet.Type_BYTE_ARRAY),
						Name:           fieldName,
						RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
					},
				}
			case reflect.Array:
				typeLen := int32(fieldType.Len())
				// Handle special case for [N]byte.
				column = &parquetschema.ColumnDefinition{
					SchemaElement: &parquet.SchemaElement{
						Type:           parquet.TypePtr(parquet.Type_FIXED_LEN_BYTE_ARRAY),
						Name:           fieldName,
						RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
						TypeLength:     &typeLen,
					},
				}
			}
		} else {
			var elementColumn *parquetschema.ColumnDefinition
			elementColumn, err = generateField(field, fieldType.Elem(), "element")
			if err != nil {
				return
			}

			repType := elementColumn.SchemaElement.RepetitionType
			elementColumn.SchemaElement.RepetitionType = parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED)
			column = &parquetschema.ColumnDefinition{
				SchemaElement: &parquet.SchemaElement{
					Name:           fieldName,
					RepetitionType: repType,
					ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_LIST),
					LogicalType: &parquet.LogicalType{
						LIST: &parquet.ListType{},
					},
				},
				Children: []*parquetschema.ColumnDefinition{
					{
						SchemaElement: &parquet.SchemaElement{
							Name:           "list",
							RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REPEATED),
						},
						Children: []*parquetschema.ColumnDefinition{
							elementColumn,
						},
					},
				},
			}
		}
	case reflect.String:
		column = &parquetschema.ColumnDefinition{
			SchemaElement: &parquet.SchemaElement{
				Type:           parquet.TypePtr(parquet.Type_BYTE_ARRAY),
				Name:           fieldName,
				RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				ConvertedType:  parquet.ConvertedTypePtr(parquet.ConvertedType_UTF8),
				LogicalType: &parquet.LogicalType{
					STRING: &parquet.StringType{},
				},
			},
		}
	case reflect.Struct:
		switch {
		case fieldType.ConvertibleTo(reflect.TypeOf(time.Time{})):
			column = &parquetschema.ColumnDefinition{
				SchemaElement: &parquet.SchemaElement{
					Type:           parquet.TypePtr(parquet.Type_INT64),
					Name:           fieldName,
					RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
					LogicalType: &parquet.LogicalType{
						TIMESTAMP: &parquet.TimestampType{
							IsAdjustedToUTC: true,
							Unit: &parquet.TimeUnit{
								NANOS: parquet.NewNanoSeconds(),
							},
						},
					},
				},
			}
		case fieldType.ConvertibleTo(reflect.TypeOf(goparquet.Time{})):
			column = &parquetschema.ColumnDefinition{
				SchemaElement: &parquet.SchemaElement{
					Type:           parquet.TypePtr(parquet.Type_INT64),
					Name:           fieldName,
					RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
					LogicalType: &parquet.LogicalType{
						TIME: &parquet.TimeType{
							IsAdjustedToUTC: true,
							Unit: &parquet.TimeUnit{
								NANOS: parquet.NewNanoSeconds(),
							},
						},
					},
				},
			}
		default:
			var children []*parquetschema.ColumnDefinition
			children, err = generateSchema(fieldType)
			if err != nil {
				return
			}
			column = &parquetschema.ColumnDefinition{
				SchemaElement: &parquet.SchemaElement{
					Name:           fieldName,
					RepetitionType: parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED),
				},
				Children: children,
			}
		}
	case reflect.UnsafePointer:
		err = errors.New("unsafe.Pointer is unsupported")
	default:
		err = errors.Errorf("unknown kind %s is unsupported", fieldType.Kind())
	}

	return
}
