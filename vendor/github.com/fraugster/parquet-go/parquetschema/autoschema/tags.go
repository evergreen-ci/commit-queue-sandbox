package autoschema

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	goparquet "github.com/fraugster/parquet-go"
	"github.com/fraugster/parquet-go/parquet"
	"github.com/fraugster/parquet-go/parquetschema"
	"github.com/pkg/errors"
)

// Struct tag field keys and prefixes.
const (
	StructTagNameKey            = "name"
	StructTagTypeKey            = "type"
	StructTagLogicalTypeKey     = "logicaltype"
	StructTagIsAdjustedToUTCKey = "isadjustedtoutc"
	StructTagTimeUnitKey        = "timeunit"
	StructTagScaleKey           = "scale"
	StructTagPrecisionKey       = "precision"
	StructTagKeyPrefix          = "key."
	StructTagValuePrefix        = "value."
	StructTagElementPrefix      = "element."
)

// CreateTagFieldMap parses the struct field's `parquet` tag and returns a map
// of struct field key to value pairs. Struct tag fields should be in the
// format `key=value` and comma-seprarated.
func CreateTagFieldMap(field reflect.StructField, prefix string) (map[string]string, error) {
	tagFieldMap := map[string]string{}
	for _, tagField := range strings.Split(field.Tag.Get("parquet"), ",") {
		splitField := strings.Split(tagField, "=")
		if len(splitField) != 2 {
			// The struct tag field does not follow the `key=val`
			// convention, skip it.
			continue
		}

		prefixedKey := strings.TrimSpace(splitField[0])
		if !strings.HasPrefix(prefixedKey, prefix) {
			// Ignore required prefix for this mapping, such as
			// "key." or "value.".
			continue
		}
		key := prefixedKey[len(prefix):]
		if _, ok := tagFieldMap[key]; ok {
			return nil, errors.Errorf("struct tag field '%s' specified more than once", prefixedKey)
		}
		tagFieldMap[key] = strings.TrimSpace(splitField[1])
	}

	return tagFieldMap, nil
}

func parseParquetTag(field reflect.StructField, columnType reflect.Type, column *parquetschema.ColumnDefinition) error {
	element := column.SchemaElement
	tagFieldMap, err := CreateTagFieldMap(field, getParquetTagPrefix(element.Name))
	if err != nil {
		return errors.Wrap(err, "creating struct tag field map")
	}

	if name, ok := tagFieldMap[StructTagNameKey]; ok {
		element.Name = name
	} else if element.Name == "" {
		element.Name = strings.ToLower(field.Name)
	}

	for len(column.Children) > 0 {
		// This is a column definition with children, just set the name
		// and return since any other struct tag fields are for the
		// children column definitions.
		return nil
	}

	if typeString, ok := tagFieldMap[StructTagTypeKey]; ok {
		element.Type, err = typeFromString(typeString)
		if err != nil {
			return errors.Wrap(err, "getting type from string")
		}
	}

	if logicalTypeString, ok := tagFieldMap[StructTagLogicalTypeKey]; ok {
		element.LogicalType, element.ConvertedType, err = logicalTypeFromString(logicalTypeString)
		if err != nil {
			return errors.Wrap(err, "getting the logical type from string")
		}

		if element.LogicalType.DATE != nil {
			// Ensure that the Parquet type is set to INT32 for
			// logical DATE fields since they may have been
			// converted from a time.Time struct field (which
			// defaults to int64).
			element.Type = parquet.TypePtr(parquet.Type_INT32)
		}
	}

	if isAdjustedToUTCString, ok := tagFieldMap[StructTagIsAdjustedToUTCKey]; ok || (element.LogicalType != nil && (element.LogicalType.TIME != nil || element.LogicalType.TIMESTAMP != nil)) {
		isAdjustedToUTC, err := isAdjustedToUTCFromString(isAdjustedToUTCString)
		if err != nil {
			return errors.Wrap(err, "getting is adjusted to UTC from string")
		}
		if element.LogicalType == nil {
			return errors.New("must specify a logical type when specifying is adjusted to UTC")
		}

		if element.LogicalType.TIME != nil {
			element.LogicalType.TIME.IsAdjustedToUTC = isAdjustedToUTC
		} else if element.LogicalType.TIMESTAMP != nil {
			element.LogicalType.TIMESTAMP.IsAdjustedToUTC = isAdjustedToUTC
		} else {
			return errors.Errorf("specifying is adjusted to UTC is incompatible with %s", parquetschema.GetSchemaLogicalType(element.LogicalType))
		}
	}

	if timeUnitString, ok := tagFieldMap[StructTagTimeUnitKey]; ok || (element.LogicalType != nil && (element.LogicalType.TIME != nil || element.LogicalType.TIMESTAMP != nil)) {
		tu, err := timeUnitFromString(timeUnitString)
		if err != nil {
			return errors.Wrap(err, "getting time unit from string")
		}
		if element.LogicalType == nil {
			return errors.New("must specify a logical type when specifying a time unit")
		}

		if element.LogicalType.TIME != nil {
			element.LogicalType.TIME.Unit = tu
			if tu.MILLIS != nil {
				element.ConvertedType = parquet.ConvertedTypePtr(parquet.ConvertedType_TIME_MILLIS)
			} else if tu.MICROS != nil {
				element.ConvertedType = parquet.ConvertedTypePtr(parquet.ConvertedType_TIME_MICROS)
			} else {
				element.ConvertedType = nil
			}
		} else if element.LogicalType.TIMESTAMP != nil {
			element.LogicalType.TIMESTAMP.Unit = tu
			if tu.MILLIS != nil {
				element.ConvertedType = parquet.ConvertedTypePtr(parquet.ConvertedType_TIMESTAMP_MILLIS)
			} else if tu.MICROS != nil {
				element.ConvertedType = parquet.ConvertedTypePtr(parquet.ConvertedType_TIMESTAMP_MICROS)
			} else {
				element.ConvertedType = nil
			}
		} else {
			return errors.Errorf("specifying a time unit is incompatible with %s", parquetschema.GetSchemaLogicalType(element.LogicalType))
		}
	}

	if scaleString, ok := tagFieldMap[StructTagScaleKey]; ok {
		scale, err := strconv.ParseInt(scaleString, 10, 32)
		if err != nil {
			return errors.Errorf("converting the specified scale value '%s' to int32", scaleString)
		}
		if element.LogicalType == nil {
			return errors.New("must specify a logical type when specifying scale")
		}
		if element.LogicalType.DECIMAL == nil {
			return errors.Errorf("specifying scale is incompatible with logical type %s", parquetschema.GetSchemaLogicalType(element.LogicalType))
		}

		element.LogicalType.DECIMAL.Scale = int32(scale)
	}

	if precisionString, ok := tagFieldMap[StructTagPrecisionKey]; ok {
		precision, err := strconv.ParseInt(precisionString, 10, 32)
		if err != nil {
			return errors.Errorf("converting the specified precision value '%s' to int32", precisionString)
		}
		if element.LogicalType == nil {
			return errors.New("must specify a logical type when specifying precision")
		}
		if element.LogicalType.DECIMAL == nil {
			return errors.Errorf("specifying precision is incompatible with %s", parquetschema.GetSchemaLogicalType(element.LogicalType))
		}

		element.LogicalType.DECIMAL.Precision = int32(precision)
	}

	// Check type compatibility at the end since there may have been some
	// additional logical after setting the Parquet type and Parquet
	// logical type that affects validity (e.g. time unit specification).
	if _, ok := tagFieldMap[StructTagTypeKey]; ok && !isTypeCompatible(element.Type, columnType) {
		return errors.Errorf("incompatible Go type %s and Parquet type %s", columnType, element.Type.String())
	}
	if _, ok := tagFieldMap[StructTagLogicalTypeKey]; ok && !isLogicalTypeCompatible(element.LogicalType, columnType) {
		return errors.Errorf("incompatible Go type %s and Parquet logical type %s", columnType, parquetschema.GetSchemaLogicalType(element.LogicalType))
	}

	return nil
}

func getParquetTagPrefix(name string) string {
	switch name {
	case "key":
		return StructTagKeyPrefix
	case "value":
		return StructTagValuePrefix
	case "element":
		return StructTagElementPrefix
	default:
		return ""
	}
}

func typeFromString(s string) (*parquet.Type, error) {
	var t parquet.Type
	switch s {
	case parquet.Type_INT96.String():
		t = parquet.Type_INT96
	default:
		return nil, errors.Errorf("unsupported type '%s' specified", s)
	}

	return parquet.TypePtr(t), nil
}

func logicalTypeFromString(s string) (*parquet.LogicalType, *parquet.ConvertedType, error) {
	var ct *parquet.ConvertedType
	lt := parquet.NewLogicalType()

	switch s {
	case "STRING":
		lt.STRING = parquet.NewStringType()
		ct = parquet.ConvertedTypePtr(parquet.ConvertedType_UTF8)
	case "ENUM":
		lt.ENUM = parquet.NewEnumType()
		ct = parquet.ConvertedTypePtr(parquet.ConvertedType_ENUM)
	case "DECIMAL":
		lt.DECIMAL = parquet.NewDecimalType()
		ct = parquet.ConvertedTypePtr(parquet.ConvertedType_DECIMAL)
	case "DATE":
		lt.DATE = parquet.NewDateType()
		ct = parquet.ConvertedTypePtr(parquet.ConvertedType_DATE)
	case "TIME":
		lt.TIME = parquet.NewTimeType()
	case "TIMESTAMP":
		lt.TIMESTAMP = parquet.NewTimestampType()
	case "JSON":
		lt.JSON = parquet.NewJsonType()
		ct = parquet.ConvertedTypePtr(parquet.ConvertedType_JSON)
	case "BSON":
		lt.BSON = parquet.NewBsonType()
		ct = parquet.ConvertedTypePtr(parquet.ConvertedType_BSON)
	case "UUID":
		lt.UUID = parquet.NewUUIDType()
	default:
		return nil, nil, errors.Errorf("unsupported logical type '%s' specified", s)
	}

	return lt, ct, nil
}

func isAdjustedToUTCFromString(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.Errorf("converting the specified is adjusted to UTC value '%s' to bool", s)
	}
}

func timeUnitFromString(s string) (*parquet.TimeUnit, error) {
	tu := parquet.NewTimeUnit()
	switch s {
	case "MILLIS":
		tu.MILLIS = parquet.NewMilliSeconds()
	case "MICROS":
		tu.MICROS = parquet.NewMicroSeconds()
	case "NANOS", "":
		tu.NANOS = parquet.NewNanoSeconds()
	default:
		return nil, errors.Errorf("unsupported time unit '%s' specified", s)
	}

	return tu, nil
}

func isTypeCompatible(pt *parquet.Type, gt reflect.Type) bool {
	if gt.Kind() == reflect.Ptr {
		gt = gt.Elem()
	}

	switch *pt {
	case parquet.Type_INT96:
		return (isByteArray(gt) && gt.Len() == 12)
	default:
		return true
	}
}

func isLogicalTypeCompatible(lt *parquet.LogicalType, gt reflect.Type) bool {
	if gt.Kind() == reflect.Ptr {
		gt = gt.Elem()
	}

	switch {
	case lt.IsSetSTRING() || lt.IsSetENUM() || lt.IsSetJSON() || lt.IsSetBSON():
		return gt.Kind() == reflect.String || isByteSlice(gt)
	case lt.IsSetDECIMAL():
		return gt.Kind() == reflect.Int32 || gt.Kind() == reflect.Int64 || isByteSlice(gt) || isByteArray(gt)
	case lt.IsSetDATE():
		return gt.Kind() == reflect.Int32 || isGoTime(gt)
	case lt.IsSetTIME():
		return (gt.Kind() == reflect.Int32 && lt.TIME.Unit.MILLIS != nil) || (gt.Kind() == reflect.Int64 && lt.TIME.Unit.MILLIS == nil) || isGoParquetTime(gt)
	case lt.IsSetTIMESTAMP():
		return gt.Kind() == reflect.Int64 || isGoTime(gt)
	case lt.IsSetUUID():
		return isByteArray(gt) && gt.Len() == 16
	default:
		return false
	}
}

func isByteSlice(gt reflect.Type) bool {
	return gt.Kind() == reflect.Slice && gt.Elem().Kind() == reflect.Uint8
}

func isByteArray(gt reflect.Type) bool {
	return gt.Kind() == reflect.Array && gt.Elem().Kind() == reflect.Uint8
}

func isGoTime(gt reflect.Type) bool {
	return gt.Kind() == reflect.Struct && gt.ConvertibleTo(reflect.TypeOf(time.Time{}))
}

func isGoParquetTime(gt reflect.Type) bool {
	return gt.Kind() == reflect.Struct && gt.ConvertibleTo(reflect.TypeOf(goparquet.Time{}))
}
