# Working with primitives

## What is a primitive

Primitive is a value-object, that behaves absolutely same like value-object, meaning that:

1. cannot be changed after creation,
2. on modification, **MUST** be copied, cloned, etc.

## Enums

Enums are implemented as a `uint8` type that have only conversion to and from string representation:

```go
type EnumType uint8

const (
	// Zero value always must be default value.
	// Use `_`, if enum does not have default value.
	_               = Protocol = iota + 1
	EnumTypeValueA   // value_a
	EnumTypeValueB   // value_b
)

// ParseEnumType parses a string into a EnumType.
func ParseEnumType(str string) (res EnumType, err error) {
	err = res.UnmarshalText([]byte(str))
	return res, err
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *EnumType) UnmarshalText(buf []byte) error {
	// ...
}

// Valid checks if the enum is valid.
func (s EnumType) Valid() bool {
	// ...
}
```

All of methods above must be generated with `stringer` tool, with minimal invasion to the real-written code.

## Composed primitives

Complex primitives are primitives that are composed of multiple other primitives, including standard types like `string`, `int`, `bool`, or even other composed primitives and enums.

For example:

```go
type ComposedPrimitive struct {
	field1 string
	field2 int
	field3 bool
	field4 EnumType
	field5 ComposedPrimitive

    _valid bool
}
```

Take a note, that ALL of fields are private.

### Construction and validation

Each composed primitive **MUST** have `New` constructor, `Validate` and `Valid` methods for checking invariants. Caller must call constructors, with no exceptions, and check validation methods after construction, when using getters.

Example:

```go
func NewComposedPrimitive(field1, field2 string) (ComposedPrimitive, error) {
	primitive := ComposedPrimitive{
		field1: field1,
		field2: field2,
	}

	if err := primitive.Validate(); err != nil {
		return ComposedPrimitive{}, err
	}

	primitive._valid = true

	return primitive, nil
}

func (c ComposedPrimitive) Valid() bool {
	return c._valid || c.Validate() == nil
}

func (c ComposedPrimitive) Validate() error {
    if c.field1 == "" {
        return errors.New("empty field1")
    }

    if c.field2 == "" {
        return errors.New("empty field2")
    }

    // etc.

    return nil
}
```

### Method receivers

For all primitives, method receivers **MUST** be value receivers, to prevent any modification of the primitive.

### Getters

Depends on each primitive, it **MAY** have different getters. But ALL of them **MUST** return copies of the internal fields, not the fields themselves. This is to prevent the primitive from being modified after creation.

Getters **MUST NOT** return any variable, that indicates validation of primitive. The caller **MUST** call `Valid()` when using getters.

> [!CAUTION]
>
> IF GETTER MIGHT THROW ERROR, THIS MEANS, THAT INVARIANTS VALIDATION IS NOT DONE PROPERLY.

Example:

```go
func (c ComposedPrimitive) Field1() string {
	return c.field1
}

func (c ComposedPrimitive) SliceField() []string {
	return slices.Clone(c.sliceField)
}

func (c ComposedPrimitive) DeepPointerField() map[string][]string {
    res := make(map[string][]string, len(c.deepPointerField))
    for k, v := range c.deepPointerField {
        res[k] = slices.Clone(v)
    }

    return res
}
```

### Modifyers

Modifyers are methods that behaves like it modifies the primitive. But in reality, it returns a new primitive with the modified field. This is to prevent the primitive from being modified after creation.

Unlike getters, Modifyers **MAY** return error, if the modification will violate the invariants of the primitive.

Example:

```go
func (c Primitive) WithField1(field1 string) (Primitive, error) {
	cloned := c
	cloned.field1 = field1

	if err := cloned.Validate(); err != nil {
		return Primitive{}, err
	}

	return cloned, nil
}
```

### Business logic methods

Primitives **MAY** have business logic methods, that are logically connected to the primitive. For example, for `AISetOfTools` primitive, which is unchangable while performing agent loop, it may have `ConvertAIResponse` method.

## Modifying primitives contract

When changing any primitive's contract, it's strictly necessary to create a migration plan, if changes are backward incompatible.

Next types are **FORBIDDEN** to change the following fields:

1. `_valid` field, it **MUST** always be the last field in the struct, separated by space.
2. `Validate() error` method, it **MUST** always be present.
3. `Valid() bool` method, it **MUST** always be present.
