<h1 align="center">parquet-go</h1>
<p align="center">
        <a href="https://github.com/fraugster/parquet-go/releases"><img src="https://img.shields.io/github/v/tag/fraugster/parquet-go.svg?color=brightgreen&label=version&sort=semver"></a>
        <a href="https://circleci.com/gh/fraugster/parquet-go/tree/master"><img src="https://circleci.com/gh/fraugster/parquet-go/tree/master.svg?style=shield"></a>
        <a href="https://goreportcard.com/report/github.com/fraugster/parquet-go"><img src="https://goreportcard.com/badge/github.com/fraugster/parquet-go"></a>
        <a href="https://codecov.io/gh/fraugster/parquet-go"><img src="https://codecov.io/gh/fraugster/parquet-go/branch/master/graph/badge.svg"/></a>
        <a href="https://godoc.org/github.com/fraugster/parquet-go"><img src="https://img.shields.io/badge/godoc-reference-blue.svg?color=blue"></a>
        <a href="https://github.com/fraugster/parquet-go/blob/master/LICENSE"><img src="https://img.shields.io/badge/license-Apache%202-blue"></a>
</p>

---

`parquet-go` is an implementation of the [Apache Parquet file format](https://github.com/apache/parquet-format)
in Go. It provides functionality to both read and write Parquet files, as well
as high-level functionality to manage the data schema of Parquet files, to
directly write Go objects to Parquet files using automatic or custom
marshalling and to read records from Parquet files into Go objects using
automatic or custom marshalling.

Parquet is a file format to store nested data structures in a flat columnar
format. By storing in a column-oriented way, it allows for efficient reading
of individual columns without having to read and decode complete rows. This
allows for efficient reading and faster processing when using the file format
in conjunction with distributed data processing frameworks like Apache Hadoop
or distributed SQL query engines like Presto and AWS Athena.

This implementation is divided into several packages. The top-level package is
the low-level implementation of the Parquet file format. It is accompanied by
the sub-packages `parquetschema` and `floor`. `parquetschema` provides
functionality to parse textual schema definitions as well as the data types to
manually or programmatically construct schema definitions. `floor` is a
high-level wrapper around the low-level package. It provides functionality to
open Parquet files to read from them or write to them using automated or custom
marshalling and unmarshalling.

## Supported Features

| Feature                                  | Read | Write | Note |
| ---------------------------------------- | ---- | ----- | ---- |
| Compression                              | Yes  | Yes   | Only GZIP and SNAPPY are supported out of the box, but it is possible to add other compressors, see below. |
| Dictionary Encoding                      | Yes  | Yes   |
| Run Length Encoding / Bit-Packing Hybrid | Yes  | Yes   | The reader can read RLE/Bit-pack encoding, but the writer only uses bit-packing. |
| Delta Encoding                           | Yes  | Yes   |
| Byte Stream Split                        | No   | No    |
| Data page V1                             | Yes  | Yes   |
| Data page V2                             | Yes  | Yes   |
| Statistics in page meta data             | No   | Yes   | Page meta data is generally not made available to users and not used by `parquet-go`. |
| Index Pages                              | No   | No    |
| Dictionary Pages                         | Yes  | Yes   |
| Encryption                               | No   | No    |
| Bloom Filter                             | No   | No    |
| Logical Types                            | Yes  | Yes   | Support for logical type is in the high-level package `floor` the low level Parquet library only supports the basic types, see the type mapping table. |

## Supported Data Types

| Parquet Type            | Go Type         | Note |
| ----------------------- | --------------- | ---- |
| BOOLEAN                 | bool            |
| INT32                   | int32           | See the note about the int type. |
| INT64                   | int64           | See the note about the int type. |
| INT96                   | [12]byte        |
| FLOAT                   | float32         |
| DOUBLE                  | float64         |
| BYTE\_ARRAY             | string, []byte  |
| FIXED\_LEN\_BYTE\_ARRAY | []byte, [N]byte |

Note: the low-level implementation only supports int32 for the INT32 type and int64 for the INT64 type in Parquet.
Plain int or uint are not supported. The high-level `floor` package contains more extensive support for these
data types.

## Supported Logical Types

| Logical Type | Mapped to Go types           | Note |
| ------------ | -----------------------      | ---- |
| STRING       | string, []byte               |
| DATE         | int32, time.Time             | int32: Days since Unix epoch (Jan 01 1970 00:00:00 UTC); time.Time: only in `floor` |
| TIME         | int32, int64, goparquet.Time | int32: TIME(MILLIS, ...), int64: TIME(MICROS, ...), TIME(NANOS, ...); goparquet.Time: only in `floor` |
| TIMESTAMP    | int64, [12]byte, time.Time   | time.Time: only in `floor` |
| UUID         | [16]byte                     |
| LIST         | []T, [N]T                    | Slices and arrays of any type. |
| MAP          | map[T1]T2                    | Maps with any key and value types. |
| ENUM         | string, []byte               |
| JSON         | string, []byte               |
| BSON         | string, []byte               |
| DECIMAL      | int32, int64,[]byte, [N]byte |
| INTEGER      | {,u}int{,8,16,32,64}         | Implementation is loose and will allow any INTEGER logical type converted to any signed or unsigned int Go type. |

## Supported Converted Types

| Converted Type        | Mapped to Go types  | Note |
| --------------------  | ------------------- | ---- |
| UTF8                  | string, []byte      |
| TIME\_MILLIS          | int32               | Number of milliseconds since the beginning of the day. |
| TIME\_MICROS          | int64               | Number of microseconds since the beginning of the day. |
| TIMESTAMP\_MILLIS     | int64               | Number of milliseconds since Unix epoch (Jan 01 1970 00:00:00 UTC). |
| TIMESTAMP\_MICROS     | int64               | Number of milliseconds since Unix epoch (Jan 01 1970 00:00:00 UTC). |
| {,U}INT\_{8,16,32,64} | {,u}int{8,16,32,64} | Implementation is loose and will allow any converted type with any int Go type. |
| INTERVAL              | [12]byte            |

Please note that converted types are deprecated. Logical types should be used preferably.

## Supported Compression Algorithms

| Compression Algorithm | Supported | Notes |
| --------------------- | --------- | ----- |
| GZIP                  | Yes; Out of the box |
| SNAPPY                | Yes; Out of the box |
| BROTLI                | Yes; By importing [github.com/akrennmair/parquet-go-brotli](https://github.com/akrennmair/parquet-go-brotli) |
| LZ4                   | No | LZ4 has been deprecated as of parquet-format 2.9.0. |
| LZ4\_RAW              | Yes; By importing [github.com/akrennmair/parquet-go-lz4raw](https://github.com/akrennmair/parquet-go-lz4raw) |
| LZO                   | Yes; By importing [github.com/akrennmair/parquet-go-lzo](https://github.com/akrennmair/parquet-go-lzo) | Uses a cgo wrapper around the original LZO implementation which is licensed as GPLv2+. |
| ZSTD                  | Yes; By importing [github.com/akrennmair/parquet-go-zstd](https://github.com/akrennmair/parquet-go-zstd) |

## Schema Definition

`parquet-go` comes with support for both textual and automatic object schema
definitions.

### Textual Schema Definitions

The sub-package `parquetschema` comes with a parser to turn the textual schema
definition into the right data type to use elsewhere to specify Parquet
schemas. The syntax has been mostly reverse-engineered from a similar format
also supported but barely documented in [Parquet's Java implementation](https://github.com/apache/parquet-mr/blob/master/parquet-column/src/main/java/org/apache/parquet/schema/MessageTypeParser.java).

For the full syntax, please have a look at the [parquetschema package Go documentation](http://godoc.org/github.com/fraugster/parquet-go/parquetschema).

Generally, the schema definition describes the structure of a message. Parquet
will then flatten this into a purely column-based structure when writing the
actual data to Parquet files.

A message consists of a number of fields. Each field either has type or is a
group. A group itself consists of a number of fields, which in turn can have
either a type or are a group themselves. This allows for theoretically
unlimited levels of hierarchy.

Each field has a repetition type, describing whether a field is required (i.e.
a value has to be present), optional (i.e. a value can be present but doesn't
have to be) or repeated (i.e. zero or more values can be present). Optionally,
each field (including groups) have an annotation, which contains a logical type
or converted type that annotates something about the general structure at this
point, e.g. `LIST` indicates a more complex list structure, or `MAP` a key-value
map structure, both following certain conventions. Optionally, a typed field
can also have a numeric field ID. The field ID has no purpose intrinsic to the
Parquet file format.

Here is a simple example of a message with a few typed fields:

```
message coordinates {
    required float64 latitude;
    required float64 longitude;
    optional int32 elevation = 1;
    optional binary comment (STRING);
}
```

In this example, we have a message with four typed fields, two of them
required, and two of them optional. `float64`, `int32` and `binary` describe
the fundamental data type of the field, while `longitude`, `latitude`,
`elevation` and `comment` are the field names. The parentheses contain
an annotation `STRING` which indicates that the field is a string, encoded
as binary data, i.e. a byte array. The field `elevation` also has a field
ID of `1`, indicated as numeric literal and separated from the field name
by the equal sign `=`.

In the following example, we will introduce a plain group as well as two
nested groups annotated with logical types to indicate certain data structures:

```
message transaction {
    required fixed_len_byte_array(16) txn_id (UUID);
    required int32 amount;
    required int96 txn_ts;
    optional group attributes {
        optional int64 shop_id;
        optional binary country_code (STRING);
        optional binary postcode (STRING);
    }
    required group items (LIST) {
        repeated group list {
            required int64 item_id;
            optional binary name (STRING);
        }
    }
    optional group user_attributes (MAP) {
        repeated group key_value {
            required binary key (STRING);
            required binary value (STRING);
        }
    }
}
```

In this example, we see a number of top-level fields, some of which are
groups. The first group is simply a group of typed fields, named `attributes`.

The second group, `items` is annotated to be a `LIST` and in turn contains a
`repeated group list`, which in turn contains a number of typed fields. When
a group is annotated as `LIST`, it needs to follow a particular convention:
it has to contain a `repeated group` named `list`. Inside this group, any
fields can be present.

The third group, `user_attributes` is annotated as `MAP`. Similar to `LIST`,
it follows some conventions. In particular, it has to contain only a single
`required group` with the name `key_value`, which in turn contains exactly two
fields, one named `key`, the other named `value`. This represents a map
structure in which each key is associated with one value.

### Object Schema Definitions

The sub-package `parquetschema/autoschema` supports auto-generating schema
definitions for a provided object's type using reflection and struct tags. The
generated schema is meant to be compatible with the reflection-based
marshalling/unmarshalling in the `floor` sub-package.

#### Supported Parquet Types

| Parquet Type            | Go Types                     | Note |
| ----------------------- | ---------------------------- | ---- |
| BOOLEAN                 | bool                         |
| INT32                   | int{8,16,32}, uint{,8,16,32} | 
| INT64                   | int{,64}, uint64             |
| INT96                   | [12]byte                     | Must specify `type=INT96` in the `parquet` struct tag. |
| FLOAT                   | float32                      |
| DOUBLE                  | float64                      |
| BYTE\_ARRAY             | string, []byte               |
| FIXED\_LEN\_BYTE\_ARRAY | []byte, [N]byte              |

#### Supported Logical Types

| Logical Type | Go Types                      | Note |
| ------------ | ----------------------------- | ---- |
| STRING       | string, []byte                |
| MAP          | map[T1]T2                     | Maps with any key and value types. |
| LIST         | []T, [N]T                     | Slices and arrays of any type except for byte. |
| ENUM         | string, []byte                |
| DECIMAL      | int32, int64, []byte, [N]byte |
| DATE         | int32, time.Time              |
| TIME         | int32, int64, goparquet.Time  | int32: TIME(MILLIS, {false,true}), int64: TIME({MICROS,NANOS}, {false,true}) |
| TIMESTAMP    | int64, time.Time              |
| INTEGER      | {,u}int{,8,16,32,64}          |
| JSON         | string, []byte                |
| BSON         | string, []byte                |
| UUID         | [16]byte                      |

Pointers are automatically mapped to optional fields. Unsupported Go types
include funcs, interfaces, unsafe pointers, unsigned int pointers, and complex
numbers.

#### Default Type Mappings

By default, Go types are mapped to Parquet types and in some cases logical
types as well. More specific mappings can be achieved by the use of struct
tags (see below).

| Go Type           | Default Parquet Type    | Default Logical Type |
| ------------------| ----------------------- | -------------------- |
| bool              | BOOLEAN                 |
| int{,8,16,32,64}  | INT{64,32,32,32,64}     | INTEGER({64,8,16,32,64}, true) |
| uint{,8,16,32,64} | INT{32,32,32,32,64}     | INTEGER({32,8,16,32,64}, false) |
| string            | BYTE\_ARRAY             | STRING |
| []byte            | BYTE\_ARRAY             |
| [N]byte           | FIXED\_LEN\_BYTE\_ARRAY |
| time.Time         | INT64                   | TIMESTAMP(NANOS, true)
| goparquet.Time    | INT64                   | TIME(NANOS, true)
| map               | group                   | MAP |
| slice, array      | group                   | LIST |
| struct            | group                   |


#### Struct Tags

Automatic schema definition generation supports the use of the `parquet` struct
tag for further schema specification beyond the default mappings. Tag fields
have the format `key=value` and are comma separated. The tags do not support
converted types as these are now deprecated by Parquet. Since converted types
are still required to support backward compatibility, they are automatically
set based on a field's logical type.

| Tag Field       | Type   | Values                                                                           | Notes |
| --------------- | ------ | -------------------------------------------------------------------------------- | ----- |
| name            | string | ANY                                                                              | Defaults to the lower-case struct field name. |
| type            | string | `INT96`                                                                          | Unless using a [12]byte field for INT96, this does not ever need to be specified. |
| logicaltype     | string | `STRING`, `ENUM`, `DECIMAL`, `DATE`, `TIME`, `TIMESTAMP`, `JSON`, `BSON`, `UUID` | Maps and non-byte slices and arrays are always mapped to MAP and LIST logical types, respectively. |
| timeunit        | string | `MILLIS`, `MICROS`, `NANOS`                                                      | Only used when the logical type is TIME or TIMESTAMP, defaults to `NANOS`. |
| isadjustedtoutc | bool   | ANY                                                                              | Only used when the logical type is TIME or TIMESTAMP, defaults to `true`. |
| scale           | int32  | N >= 0                                                                           | Only used when the logical type is DECIMAL, defaults to 0. |
| precision       | int32  | N >= 0                                                                           | Only used when the logical type is DECIMAL, required. |

All fields must be prefixed by `key.` and `value.` when referring to key and
value types of a map, respectively, and `element.` when referring to the
element type of a slice or array. It is invalid to prefix `name` since it can
only apply to the field itself.

#### Object Schema Example

```
type example  struct {
        ByteSlice          []byte
        String             string
        ByteString         []byte          `parquet:"name=byte_string, logicaltype=STRING"`
        Int64              int64           `parquet:"name=int_64"`
        Uint8              uint8           `parquet:"name=u_int_8"`
        Int96              [12]byte        `parquet:"name=int_96, type=INT96"`
        DefaultTS          time.Time       `parquet:"name=default_ts"`
        Timestamp          int64           `parquet:"name=ts, logicaltype=TIMESTAMP, timeunit=MILLIS, isadjustedtoutc=false`
        Date               time.Time       `parquet:"name=date, logicaltype=DATE"`
        OptionalDecimal    *int32          `parquet:"name=decimal, logicaltype=DECIMAL, scale=5, precision=10"`
        TimeList           []int32         `parquet:"name=time_list, element.logicaltype=TIME, element.timeunit=MILLIS"`
	DecimalTimeMap     map[int64]int32 `parquet:"name=decimal_time_map, key.logicaltype=DECIMAL, key.scale=5, key.precision=15, value.logicaltype=TIME, value.timeunit=MILLIS", value.isadjustedtoutc=true`
        Struct             struct {
                OptionalInt64 *int64   `parquet:"name=int_64"`
	        Time          int64    `parquet:"name=time, logicaltype=TIME, isadjustedtoutc=false"`
	        StringList    []string `parquet:"name=string_list"`
        } `parquet:"name=struct"`
}
```

The above struct is equivalent to the following schema definition:

```
message autogen_schema {
    required binary byteslice;
    required binary string (STRING);
    required binary byte_string (STRING);
    required int64 int_64 (INTEGER(64,true));
    required int32 int_8 (INTEGER(8,false));
    required int96 int_96;
    required int64 default_ts (TIMESTAMP(NANOS,true));
    required int64 ts (TIMESTAMP(MILLIS,false));
    required int32 date (DATE);
    optional int32 decimal (DECIMAL(10,5));
    required group time_list (LIST) {
        repeated group list {
          required int32 element (TIME(MILLIS,true));
        }
    }
    optional group decimal_time_map (MAP) {
        repeated group key_value (MAP_KEY_VALUE) {
          required int64 key (DECIMAL(15,5));
          required int32 value (TIME(MILLIS, true));
        }
    }
    required group struct {
        optional int64 int_64 (INTEGER(64,true));
        required int64 time (TIME(NANOS, false));
        required group string_list (LIST) {
            repeated group list {
                required binary element (STRING);
            }
        }
    }
}
```

## Examples

For examples how to use both the low-level and high-level APIs of this library, please
see the directory `examples`. You can also check out the accompanying tools (see below)
for more advanced examples. The tools are located in the `cmd` directory.

## Tools

`parquet-go` comes with tooling to inspect and generate Parquet files.

### parquet-tool

`parquet-tool` allows you to inspect the meta data, the schema and the number of rows
as well as print the content of a Parquet file. You can also use it to split an existing
Parquet file into multiple smaller files.

Install it by running `go get github.com/fraugster/parquet-go/cmd/parquet-tool` on your command line.
For more detailed help on how to use the tool, consult `parquet-tool --help`.

### csv2parquet

`csv2parquet` makes it possible to convert an existing CSV file into a Parquet file. By default,
all columns are simply turned into strings, but you provide it with type hints to influence
the generated Parquet schema.

You can install this tool by running `go get github.com/fraugster/parquet-go/cmd/csv2parquet` on your command line.
For more help, consult `csv2parquet --help`.

## Contributing

If you want to hack on this repository, please read the short [CONTRIBUTING.md](CONTRIBUTING.md)
guide first.

# Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available,
see the [tags on this repository][tags].

## Authors

- **Forud Ghafouri** - *Initial work* [fzerorubigd](https://github.com/fzerorubigd)
- **Andreas Krennmair** - *floor package, schema parser* [akrennmair](https://github.com/akrennmair)
- **Stefan Koshiw** - *Engineering Manager for Core Team* [panamafrancis](https://github.com/panamafrancis)

See also the list of [contributors][contributors] who participated in this project.

## Special Mentions

- **Nathan Hanna** - *proposal and prototyping of automatic schema generator* [jnathanh](https://github.com/jnathanh)

## License

Copyright 2021 Fraugster GmbH

This project is licensed under the Apache-2 License - see the [LICENSE](LICENSE) file for details.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. 

[tags]: https://github.com/fraugster/parquet-go/tags
[contributors]: https://github.com/fraugster/parquet-go/graphs/contributors

