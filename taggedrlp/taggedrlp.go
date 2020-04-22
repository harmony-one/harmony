// Package taggedrlp adds support for tagged alternative types with legacy
// fallback support.
//
// A tagged item is encoded and decoded as a signature-tag-value tuple,
// except an item of the “legacy” type – which has the empty tag – is encoded
// verbatim without the sig-tag-value envelope.
// Legacy type support makes it possible to use this package as a drop-in
// replacement for a non-tagged type.
package taggedrlp

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/harmony-one/harmony/rlp"
)

// Tag is a string used to distinguish alternative types.
type Tag string

// LegacyTag is a special type tag used to denote legacy type which should be
// encoded/decoded without envelope.
const LegacyTag Tag = ""

// Factory is a new-instance factory function.
// A factory is associated with a particular tagged type,
// and returns a new instance of the type, ready to be filled by RLP decoder.
// For example, a struct type with a *big.Int pointer field needs a factory to
// return a new instance with the pointer allocated as the RLP decoder
// expects the field not to be nil but to point to a valid *big.Int object.
type Factory = func() interface{}

type (
	typeForTag     map[Tag]reflect.Type
	tagForType     map[reflect.Type]Tag
	factoryForType map[reflect.Type]Factory
)

// Registry maintains a tag-to-type mapping.
// Tagged alternative types that can be interchangeably used should be
// registered within the same registry.
type Registry struct {
	typeForTag     typeForTag
	tagForType     tagForType
	factoryForType factoryForType
}

// NewRegistry creates a new registry.
func NewRegistry() *Registry {
	return &Registry{
		typeForTag:     typeForTag{},
		tagForType:     tagForType{},
		factoryForType: factoryForType{},
	}
}

// PkgPathQualifiedName returns a package-path-qualified name.
// If the package path is empty, it returns just the name; otherwise,
// it returns a qualified name such as "math/big".Int.
func PkgPathQualifiedName(name, pkgPath string) string {
	if pkgPath != "" {
		return fmt.Sprintf("\"%s\".%s", pkgPath, name)
	}
	return name
}

// PkgPathQualifiedTypeName returns a package-path-qualified name of the
// given type, or an empty string if the type name is not available,
// e.g. internal or unexported type.
func PkgPathQualifiedTypeName(typ reflect.Type) string {
	return PkgPathQualifiedName(typ.Name(), typ.PkgPath())
}

// TypeName returns a package-path-qualified name of the given type,
// or a package-name-qualified name if the package path is not available
// through reflection.
func TypeName(typ reflect.Type) string {
	name := PkgPathQualifiedTypeName(typ)
	if name == "" {
		name = typ.String()
	}
	return name
}

// Register adds a new tagged type to the registry.
// The type is specified by a prototype value of that type.
// Its value does not matter,
// so one can simply pass in a zero value of the right type.
//
// Registering the same tag-type pair is an idempotent operation.  However,
// it is an error to attempt to add the same type under multiple tags,
// or to add another type for the same tag.
func (reg *Registry) Register(tag Tag, prototype interface{}) error {
	typ := reflect.TypeOf(prototype)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	typ2, typFound := reg.typeForTag[tag]
	typChanged := typFound && (typ != typ2)
	tag2, tagFound := reg.tagForType[typ]
	tagChanged := tagFound && (tag != tag2)
	switch {
	case typFound && tagFound && !typChanged && !tagChanged:
		// Idempotent
		return nil
	case typChanged:
		return TagAlreadyBoundToType{tag, typ2}
	case tagChanged:
		return TypeAlreadyBoundToTag{typ, tag2}
	}
	reg.typeForTag[tag] = typ
	reg.tagForType[typ] = tag
	return nil
}

// TagAlreadyBoundToType indicates that a tag is already bound to a type and
// cannot be rebound to another type.
type TagAlreadyBoundToType struct {
	Tag  Tag
	Type reflect.Type
}

// Error returns a formatted error string.
func (e TagAlreadyBoundToType) Error() string {
	return fmt.Sprintf("tag %#v is already bound to type %s",
		e.Tag, TypeName(e.Type))
}

// TypeAlreadyBoundToTag indicates that a type is already bound to a tag and
// cannot be rebound to another tag.
type TypeAlreadyBoundToTag struct {
	Type reflect.Type
	Tag  Tag
}

// Error returns a formatted error string.
func (e TypeAlreadyBoundToTag) Error() string {
	return fmt.Sprintf("type %v is already bound to tag %#v",
		TypeName(e.Type), e.Tag)
}

// MustRegister is like Register, but panics on error.  Use only in init().
func (reg *Registry) MustRegister(tag Tag, prototype interface{}) {
	if err := reg.Register(tag, prototype); err != nil {
		panic(err)
	}
}

// AddFactory adds the given custom factory function.
// By default, when an instance of a tagged type is needed when decoding,
// the decoded calls the new() function with the type to create a zero-valued
// instance.  Sometimes, a zero-valued instance is inadequate,
// e.g. when the type contains a big.Int field.
// A custom factory function addresses this limitation by returning a
// suitably initialized instance ready to be filled in by the RLP decoder.
//
// AddFactory exercises the given factory once to figure out the return value
// type with which to associate the factory.
// The returned value must be a pointer to a registered type.
func (reg *Registry) AddFactory(factory Factory) error {
	typ := reflect.TypeOf(factory())
	if typ.Kind() != reflect.Ptr {
		return InvalidFactoryReturnType{typ}
	}
	typ = typ.Elem()
	if _, ok := reg.tagForType[typ]; !ok {
		return TypeNotRegistered{typ}
	}
	if _, ok := reg.factoryForType[typ]; ok {
		return TypeAlreadyHasFactory{typ}
	}
	reg.factoryForType[typ] = factory
	return nil
}

// InvalidFactoryReturnType indicates that the given factory does not return
// a pointer type.
type InvalidFactoryReturnType struct {
	Type reflect.Type
}

// Error returns a formatted error string.
func (e InvalidFactoryReturnType) Error() string {
	return fmt.Sprintf("factory returns a value of a non-pointer type %s",
		TypeName(e.Type))
}

// TypeNotRegistered indicates that the given type is not registered.
type TypeNotRegistered struct {
	Type reflect.Type
}

// Error returns a formatted error string.
func (e TypeNotRegistered) Error() string {
	return fmt.Sprintf("type %s is not registered", TypeName(e.Type))
}

// TypeAlreadyHasFactory indicates that the given type already has a factory.
type TypeAlreadyHasFactory struct {
	Type reflect.Type
}

// Error returns a formatted error string.
func (e TypeAlreadyHasFactory) Error() string {
	return fmt.Sprintf("type %s already has a factory", TypeName(e.Type))
}

// MustAddFactory is like AddFactory, but panics on error.  Use only in init().
func (reg *Registry) MustAddFactory(factory Factory) {
	if err := reg.AddFactory(factory); err != nil {
		panic(err)
	}
}

// EnvelopeSignature is the first item in a tagged envelope.
const EnvelopeSignature = "HmnyTgd"

// Envelope is the tagged envelope type.
type Envelope struct {
	Sig string
	Tag Tag
	Raw rlp.RawValue
}

// Encode encodes the given value.
func (reg *Registry) Encode(w io.Writer, d interface{}) error {
	typ := reflect.TypeOf(d)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	tag, ok := reg.tagForType[typ]
	if !ok {
		return TypeNotRegistered{typ}
	}
	bw := bytes.NewBuffer(nil)
	if err := rlp.Encode(bw, d); err != nil {
		return UnencodableValue{d, err}
	}
	b := rlp.RawValue(bw.Bytes())
	if tag == LegacyTag {
		// Legacy type; emit as is
		d = b
	} else {
		// Tagged type; wrap in an envelope
		d = &Envelope{EnvelopeSignature, tag, b}
	}
	if err := rlp.Encode(w, d); err != nil {
		// This path is untestable unless we inject rlp.Encode failure.
		return UnencodableValue{d, err}
	}
	return nil
}

// UnencodableValue indicates that the given value cannot be RLP-encoded.
type UnencodableValue struct {
	Value interface{}
	Err   error
}

// Error returns a formatted error string.
func (e UnencodableValue) Error() string {
	return fmt.Sprintf("value %#v cannot be encoded: ", e.Value) + e.Err.Error()
}

// Cause returns the underlying encoding error.
func (e UnencodableValue) Cause() error {
	return e.Err
}

// Decode decodes the given value.
func (reg *Registry) Decode(s *rlp.Stream) (interface{}, error) {
	value, err := s.Raw()
	if err != nil {
		return nil, err
	}
	tag := LegacyTag
	var e Envelope
	err = rlp.DecodeBytes(value, &e)
	if err == nil && e.Sig == EnvelopeSignature && e.Tag != "" {
		value = e.Raw
		tag = e.Tag
	}
	typ, ok := reg.typeForTag[tag]
	if !ok {
		return nil, UnsupportedTag{tag}
	}
	var obj interface{}
	if factory, ok := reg.factoryForType[typ]; ok {
		obj = factory()
	} else {
		obj = reflect.New(typ).Interface()
	}
	if err = rlp.DecodeBytes(value, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// UnsupportedTag indicates that an unsupported tag was encountered in stream.
type UnsupportedTag struct {
	Tag Tag
}

// Error returns a formatted error string.
func (e UnsupportedTag) Error() string {
	return fmt.Sprintf("unsupported tag %#v", e.Tag)
}
